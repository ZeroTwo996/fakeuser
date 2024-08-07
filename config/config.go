package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

var (
	RECORDENABLED      = true            // 是否开启定时记录任务，默认开启
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
	STARTTIME         string // 测试开始时间
	ACCELERATIONRATIO int    // 加速比例
	SCALERATIO        int    // 缩放比例
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

	STARTTIME = os.Getenv("START_TIME")
	if STARTTIME == "" {
		log.Fatalf("Failed to get start time from env")
	}

	RECORDENABLEDSTR := os.Getenv("USERCENTER_RECORD_ENABLED")
	if RECORDENABLEDSTR != "" {
		RECORDENABLED = strings.EqualFold(RECORDENABLEDSTR, "false") // 与用户交互模块的定时记录任务开关相反
	}

	var err error
	ACCELERATIONRATIO, err = strconv.Atoi(os.Getenv("ACCELERATION_RATIO"))
	if err != nil {
		log.Fatal("Failed to get acceleration ratio from env")
	} else if ACCELERATIONRATIO <= 0 {
		log.Fatal("Acceleration ratio must be positive")
	}

	SCALERATIO, err = strconv.Atoi(os.Getenv("SCALE_RATIO"))
	if err != nil {
		log.Fatal("Failed to get instance scale ratio from env")
	} else if SCALERATIO <= 0 {
		log.Fatal("Instance scale ratio must be positive")
	}

}
