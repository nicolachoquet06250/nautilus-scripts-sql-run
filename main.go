package main

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/ncruces/zenity"
)

type DBType string

const (
	MYSQL DBType = "MySQL"
	MARIA DBType = "MariaDB"
	POSTG DBType = "PostgreSQL"
)

type DBCredentials struct {
	username string
	password string
	host     string
}

var dbType string

var dbTypeValues = map[DBType]string{
	MYSQL: "mysql",
	MARIA: "mysql",
	POSTG: "postgres",
}

var dbTypeCnxStringFormats = map[DBType]func(string, string, string, string) string{
	MYSQL: func(username string, password string, host string, dbname string) string {
		if host != "" {
			host = fmt.Sprintf("tcp(%s)", host)
		}
		return fmt.Sprintf("%s:%s@%s/%s", username, password, host, dbname)
	},
	MARIA: func(username string, password string, host string, dbname string) string {
		if host != "" {
			host = fmt.Sprintf("tcp(%s)", host)
		}
		return fmt.Sprintf("%s:%s@%s/%s", username, password, host, dbname)
	},
	POSTG: func(username string, password string, host string, dbname string) string {
		return fmt.Sprintf("postgres://%v:%v@%v/%v?sslmode=disable", username, password, host, dbname)
	},
}

func selectSqlFiles(files []string) (final []string) {
	for _, file := range files {
		if strings.HasSuffix(file, ".sql") {
			final = append(final, file)
		}
	}
	return
}

func cleanQueries(queries []string) (final []string) {
	for _, query := range queries {
		if strings.Trim(query, "\n") != "" {
			final = append(final, strings.Trim(query, "\n"))
		}
	}
	return
}

func idExists[T comparable](id int, items []T) bool {
	for i := range items {
		if i == id {
			return true
		}
	}
	return false
}

func getDbType(content string) (dbType string, finalContent string) {
	matches := regexp.MustCompile(`(?m)^/[*]+[\s*]*Database:\s*(MySQL|MariaDB|PostgresSQL)[\s*]+/`).
		FindStringSubmatch(content)

	if idExists(1, matches) {
		dbType = matches[1]

		finalContent = strings.Trim(strings.ReplaceAll(content, matches[0], ""), "\n")
	}

	return
}

func explodeQueries(content string) []string {
	return cleanQueries(
		strings.Split(content, ";"),
	)
}

func inputHost(label, title, defaultValue string) (host string) {
	host, _ = zenity.Entry(
		label,
		zenity.Title("Run SQL - "+title),
		zenity.EntryText(defaultValue),
	)
	return
}

func inputDbType(label string, items ...string) (dbTypeOut string) {
	if dbType == "" {
		dbTypeOut, _ = zenity.ListItems(label, items...)
	} else {
		dbTypeOut = dbType
	}
	return
}

func inputLoginData(title string) (username, password string) {
	username, password, _ = zenity.Password(
		zenity.Title("Run SQL - "+title),
		zenity.Username(),
	)

	if username != "" && password == "" {
		password = username
	}

	return
}

func notifySuccess(message string) {
	_ = zenity.Notify(
		message,
		zenity.Title("Run SQL"),
		zenity.InfoIcon,
	)
}

func openDatabase(username, password, host, dbname string) *sql.DB {
	db, err := sql.Open(
		dbTypeValues[DBType(dbType)],
		dbTypeCnxStringFormats[DBType(dbType)](username, password, host, dbname),
	)
	if err != nil {
		_ = zenity.Error(
			fmt.Sprintf("Connexion à la base de données impossible: %s", err.Error()),
			zenity.Title("Run SQL - Erreur"),
		)
		return nil
	}
	return db
}

func execQuery(db *sql.DB, query string) (result *sql.Result) {
	var r, err = db.Exec(query)
	if err != nil {
		_ = zenity.Error(
			fmt.Sprintf("Execution de la requête SQL impossible: %s", err.Error()),
			zenity.Title("Run SQL - Erreur"),
		)
		return nil
	}
	return &r
}

func recreateDatabaseConnexionWhenUseInstruction(query string, credentials DBCredentials) (db *sql.DB) {
	if strings.HasPrefix(query, "USE ") {
		name := strings.Trim(strings.TrimPrefix(query, "USE "), "`")
		db = openDatabase(credentials.username, credentials.password, credentials.host, name)
	}
	return
}

func main() {
	var args = selectSqlFiles(os.Args[1:])

	if len(args) == 0 {
		return
	}

	var content, _ = os.ReadFile(args[0])
	var strContent = string(content)

	dbType, strContent = getDbType(strContent)

	queries := explodeQueries(strContent)

	var host = inputHost(
		"Saisissez le domaine de la base de données :",
		"Domain", "localhost",
	)

	dbType = inputDbType(
		"De quelle type est la base de données ?",
		string(MYSQL), string(MARIA), string(POSTG),
	)

	var username, password = inputLoginData("Connexion")

	db := openDatabase(username, password, host, "")
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			_ = zenity.Error(
				err.Error(),
				zenity.Title("Run SQL - Erreur"),
			)
		}
	}(db)

	for _, query := range queries {
		if database := recreateDatabaseConnexionWhenUseInstruction(
			query,
			DBCredentials{username, password, host},
		); database != nil {
			db = database
		}

		if r := execQuery(db, query); r == nil {
			return
		}
	}

	notifySuccess("Votre script SQL " + args[0] + " as été executé avec succès.")
}
