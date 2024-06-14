package config

import (
	"log"
	"os"
	"strconv"
)

var (
	LOGINPATH          = "device/login"  // 登录接口路径
	LOGOUTPATH         = "device/logout" // 登出接口路径
	USERCENTERPROTOCOL = "http"          // 用户交互模块协议
	MAXHISTORYNUMBER   = 180             // 预测算法输入长度

	MYSQLHOST         string // MYSQL服务地址
	MYSQLPORT         string // MYSQL服务端口
	MYSQLUSER         string // MYSQL服务用户
	MYSQLPASSWORD     string // MYSQL服务密码
	MYSQLDATABASE     string // MYSQL服务数据库
	USERCENTERHOST    string // 用户交互模块地址
	USERCENTERPORT    string // 用户交互模块端口
	ACCELERATIONRATIO int
	STARTTIME         string
)

func init() {
	MYSQLHOST = os.Getenv("MYSQL_SERVICE_SERVICE_HOST")
	if MYSQLHOST == "" {
		log.Fatalf("Failed to get mysql host from env")
	}

	MYSQLPORT = os.Getenv("MYSQL_SERVICE_SERVICE_PORT")
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

	USERCENTERHOST = os.Getenv("USERCENTER_SERVICE_SERVICE_HOST")
	if USERCENTERHOST == "" {
		log.Fatalf("Failed to get usercenter host from env")
	}

	USERCENTERPORT = os.Getenv("USERCENTER_SERVICE_SERVICE_PORT")
	if USERCENTERPORT == "" {
		log.Fatalf("Failed to get usercenter port from env")
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
