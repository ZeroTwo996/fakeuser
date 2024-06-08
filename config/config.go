package config

import (
	"log"
	"os"
	"strconv"
)

var (
	MYSQLHOST     string // MYSQL服务地址
	MYSQLPORT     string // MYSQL服务端口
	MYSQLUSER     string // MYSQL服务用户
	MYSQLPASSWORD string // MYSQL服务密码
	MYSQLDATABASE string // MYSQL服务数据库

	// MYSQLHOST     = "localhost"
	// MYSQLPORT     = "3306"
	// MYSQLUSER     = "root"
	// MYSQLPASSWORD = "1435"
	// MYSQLDATABASE = "huawei"

	// MYSQLHOST     = "10.10.103.51"
	// MYSQLPORT     = "30565"
	// MYSQLUSER     = "root"
	// MYSQLPASSWORD = "cloudgame"
	// MYSQLDATABASE = "cloudgame"

	UserCenterIP   = "10.10.103.51"
	UserCenterPort = "30127"
	Protocol       = "http"
	LoginPath      = "device/login"
	LogoutPath     = "device/logout"

	ACCELERATIONRATIO int

	MAXHISTORYNUMBER = 180

	STARTTIME string
)

func init() {
	MYSQLHOST = os.Getenv("MYSQL_HOST")
	if MYSQLHOST == "" {
		log.Fatalf("Failed to get mysql host from env")
	}

	MYSQLPORT = os.Getenv("MYSQL_PORT")
	if MYSQLPORT == "" {
		log.Fatalf("Failed to get mysql port from env")
	}

	MYSQLUSER = os.Getenv("MYSQL_USER")
	if MYSQLUSER == "" {
		log.Fatalf("Failed to get mysql user from env")
	}

	MYSQLPASSWORD = os.Getenv("MYSQL_PASSWORD")
	if MYSQLPASSWORD == "" {
		log.Fatalf("Failed to get mysql password from env")
	}

	MYSQLDATABASE = os.Getenv("MYSQL_DATABASE")
	if MYSQLDATABASE == "" {
		log.Fatalf("Failed to get mysql database from env")
	}

	var err error
	ACCELERATIONRATIO, err = strconv.Atoi(os.Getenv("ACCELERATION_RATIO"))
	if err != nil {
		log.Fatal("Failed to get acceleration ratio from env")
	} else if ACCELERATIONRATIO == 0 {
		log.Fatal("Acceleration ratio cannot be zero")
	}

	STARTTIME = os.Getenv("START_TIME")
}
