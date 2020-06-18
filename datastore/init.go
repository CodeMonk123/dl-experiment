package datastore

import (
	"database/sql"
	"fmt"
	"github.com/nemoworks/dl-experiment/utils"
)
import _ "github.com/go-sql-driver/mysql"

func InitDBClient() (*sql.DB, error) {
	dbSource := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", utils.SqlHost, utils.SqlPort, utils.SqlUser, utils.SqlPasswd, utils.SqlDB)
	return sql.Open("mysql", dbSource)
}
