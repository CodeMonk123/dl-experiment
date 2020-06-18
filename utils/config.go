package utils

import (
	"fmt"
	"os"
)

// Sql config
var SqlUser string
var SqlPasswd string
var SqlHost string
var SqlPort string
var SqlDB string

func ReadSqlConfig() {
	SqlUser = os.Getenv("MYSQL_USER")
	SqlDB = os.Getenv("MYSQL_DB")
	SqlHost = os.Getenv("MYSQL_HOST")
	SqlPasswd = os.Getenv("MYSQL_PASSWD")
	SqlPort = os.Getenv("MYSQL_PORT")
	fmt.Printf("Read mysql config: %s, %s, %s, %s\n", SqlHost, SqlPort, SqlDB, SqlUser)
}
