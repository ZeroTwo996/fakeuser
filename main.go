package main

import (
	"encoding/json"
	"fakeuser/config"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	dbservice "fakeuser/database/service"

	_ "github.com/go-sql-driver/mysql"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type Device struct {
	DeviceID string
	ZoneID   string
	SiteID   string
	Host     string
	Port     int
}

type Record struct {
	SiteID    string
	Date      string
	Instances int
}

type Response struct {
	StatusCode int             `json:"status_code"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
}

type Instance struct {
	ZoneID     string `json:"zone_id"`
	SiteID     string `json:"site_id"`
	ServerIP   string `json:"server_ip"`
	InstanceID string `json:"instance_id"`
	PodName    string `json:"pod_name"`
	Port       int    `json:"port"`
	IsElastic  int    `json:"is_elastic"`
	Status     string `json:"status"`
	DeviceId   string `json:"device_id"`
}

type DeviceLoginResponse struct {
	Instance *Instance `json:"instance"` // 假设Instance是字符串类型，根据实际情况调整
}

var (
	onlineDevices sync.Map
	template      = "2006-01-02 15:04:00"
)

func main() {
	// 获取开始时间，如果配置了就从配置时间开始，否则从数据库读取最早的数据
	var (
		startTimeStr string
		err          error
	)
	if config.STARTTIME == "" {
		startTimeStr, err = dbservice.GetFirstRecord()
		if err != nil {
			log.Printf("Failed to get first record from database: %v\n", err)
		}
	} else {
		startTimeStr = config.STARTTIME
	}
	startTime, err := time.Parse(template, startTimeStr)
	if err != nil {
		log.Println("Failed to parse start time: ", err)
		return
	}

	// 获取结束时间
	endTimeStr, err := dbservice.GetLastRecord()
	if err != nil {
		log.Printf("Failed to get last record from database: %v\n", err)
	}
	endTime, err := time.Parse(template, endTimeStr)
	if err != nil {
		log.Println("Failed to parse end time: ", err)
		return
	}
	// 定时任务
	var preTime = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	var prevRecords map[string]int
	ticker := time.NewTicker(time.Duration(60*1000/config.ACCELERATIONRATIO) * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		log.Println("----------------------------------")
		var curTime time.Time
		var currRecords map[string]int

		if preTime.Year() == 9999 {
			// 说明是刚启动程序，从开始时间获取记录
			curTime = startTime
		} else if preTime != endTime {
			// 还没到结束时间
			curTime = preTime.Add(time.Minute)
		} else {
			// 到结束时间，停止循环
			break
		}
		currRecords, err = dbservice.GetRecordWithDate(curTime.Format(template))
		if err != nil {
			fmt.Printf("Failed to get record at %s: %v\n", startTimeStr, err)
			break
		}

		for siteID, currInstances := range currRecords {
			log.Println("**********************************")
			if prevRecords == nil {
				// 首次需要特殊处理，直接登录相应数量用户
				deviceLogin("huadong", siteID, currInstances, true)
			} else {
				// 余下只需要跟上一分钟对比
				prevInstances, ok := prevRecords[siteID]
				if !ok {
					log.Printf("No previous record found for site %s", siteID)
					continue
				}
				diff := currInstances - prevInstances
				if diff > 0 {
					deviceLogin("huadong", siteID, diff, false)

				} else if diff < 0 {
					deviceLogout(-diff, "huadong", siteID)
				}
			}

			dbservice.InsertRecord("huadong", siteID, curTime.Format(template), currInstances)
			log.Println()
		}
		log.Printf("Current online device count: %d", getCurrentOnlineDeviceCount())

		prevRecords = currRecords
		preTime = curTime

		log.Println()
		log.Println()
	}
}

func deviceLogin(zoneID string, siteID string, num int, isFirst bool) {
	var wg sync.WaitGroup
	var devices []string

	for i := 0; i < num; i++ {
		wg.Add(1)
		// 异步调用登录接口
		go func(isFirst bool) {
			defer wg.Done()
			var device Device
			// 获取一个不重复device_id
			for {
				randomId := uuid.NewUUID()
				deviceID := fmt.Sprintf("Dev-%s", randomId)
				if _, exist := onlineDevices.Load(deviceID); !exist {
					device = Device{ZoneID: zoneID, SiteID: siteID, DeviceID: deviceID}
					break
				}
			}

			// 向用户交互模块发出登录请求，获取实例信息
			instance, err := sendLoginRequest(device.DeviceID, device.ZoneID, device.SiteID)
			if err != nil {
				fmt.Printf("Failed to log in: %v", err)
			}
			if instance == nil {
				fmt.Println("The value of instance is nil")
				return
			}

			// 连接具体实例
			err = sendConnectRequest(device.DeviceID, instance.ServerIP, instance.Port)
			if err != nil {
				fmt.Printf("Failed to connect instance: %v\n", err)
				return
			}

			device.Host = instance.ServerIP
			device.Port = instance.Port
			devices = append(devices, device.DeviceID)
			onlineDevices.Store(device.DeviceID, device)
		}(isFirst)
	}

	wg.Wait()

	if isFirst {
		log.Printf("Toal %d devices logged in %s, %s", len(devices), siteID, zoneID)
	} else {
		for _, id := range devices {
			log.Println(id)
		}
		log.Printf("%d devices logged in %s, %s", len(devices), siteID, zoneID)
	}
}

func deviceLogout(num int, zoneID string, siteID string) {

	// 1. 选择num个在对应zone和site的设备
	var devices []string
	onlineDevices.Range(func(key, value interface{}) bool {
		device := value.(Device)
		if device.ZoneID == zoneID && device.SiteID == siteID {
			devices = append(devices, device.DeviceID)
		}
		// 记录了足够的设备就退出遍历
		if len(devices) >= num {
			return false
		}
		return true
	})

	// 2. 将选中的设备调用登出接口，并在map中删除
	var devicesLoggedOut []string
	for _, deviceID := range devices {
		value, _ := onlineDevices.Load(deviceID)
		device := value.(Device)

		// 2.1 向实例发出断开连接请求
		err := sendDisConnectRequest(device.Host, device.Port)
		if err != nil {
			fmt.Printf("Failed to disconnect instance: %v\n", err)
			continue
		}

		// 2.2 向用户交互模块发出登出请求
		err = sendLogoutRequest(device.DeviceID, device.ZoneID)
		if err != nil {
			fmt.Printf("Failed to log out from usercenter: %v\n", err)
			continue
		}

		onlineDevices.Delete(deviceID)
		log.Println(deviceID)
		devicesLoggedOut = append(devicesLoggedOut, deviceID)

	}
	log.Printf("%d devices logged out in %s, %s", len(devicesLoggedOut), siteID, zoneID)
}

func getCurrentOnlineDeviceCount() int {
	count := 0
	onlineDevices.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func sendConnectRequest(deviceId string, host string, port int) error {
	urlStr := fmt.Sprintf("http://%s:%d/connect", host, port)
	resp, err := http.PostForm(urlStr, url.Values{
		"device_id": {deviceId},
	})
	if err != nil {
		return fmt.Errorf("failed to send connect request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to connect with status code: %v", resp.StatusCode)
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	return nil
}

func sendDisConnectRequest(host string, port int) error {
	urlStr := fmt.Sprintf("http://%s:%d/disconnect", host, port)
	resp, err := http.Get(urlStr)
	if err != nil {
		return fmt.Errorf("failed to send connect request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to connect with status code: %v", resp.StatusCode)
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return nil
}

func sendLoginRequest(deviceId string, zoneId string, siteId string) (*Instance, error) {
	urlStr := fmt.Sprintf("%s://%s:%s/%s", config.Protocol, config.UserCenterIP, config.UserCenterPort, config.LoginPath)
	resp, err := http.PostForm(urlStr, url.Values{
		"zone_id":   {zoneId},
		"site_id":   {siteId},
		"device_id": {deviceId},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to login with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response Response
	err = json.Unmarshal([]byte(string(body)), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse respinse body: %w", err)
	}

	var deviceLoginResponse DeviceLoginResponse
	err = json.Unmarshal(response.Data, &deviceLoginResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response instance data: %w", err)
	}

	return deviceLoginResponse.Instance, nil
}

func sendLogoutRequest(deviceId string, zoneId string) error {
	urlStr := fmt.Sprintf("%s://%s:%s/%s", config.Protocol, config.UserCenterIP, config.UserCenterPort, config.LogoutPath)
	resp, err := http.PostForm(urlStr, url.Values{
		"zone_id":   {zoneId},
		"device_id": {deviceId},
	})
	if err != nil {
		return fmt.Errorf("failed to send logout request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to log out with status code: %d", resp.StatusCode)
	}

	// 使用io.ReadAll读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var response Response
	err = json.Unmarshal([]byte(string(body)), &response)
	if err != nil {
		return fmt.Errorf("failed to parse response body: %w", err)
	}
	return nil
}
