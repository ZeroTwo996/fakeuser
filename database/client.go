package database

import (
	"database/sql"
	"fakeuser/config"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

var (
	DB *sql.DB
)

func init() {
	initMySQL()
}

func initMySQL() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", config.MYSQLUSER, config.MYSQLPASSWORD, config.MYSQLHOST, config.MYSQLPORT, config.MYSQLDATABASE)
	DB, _ = sql.Open("mysql", dsn)
	DB.SetConnMaxLifetime(100)
	DB.SetMaxIdleConns(1000)
	DB.SetMaxOpenConns(2000)
	//验证连接
	if err := DB.Ping(); err != nil {
		log.Println("Failed to connect to MySQL:", err)
		return
	}
	fmt.Println("MySQL connect success")
}
