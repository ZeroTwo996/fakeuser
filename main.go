package main

import (
	"encoding/json"
	"fakeuser/config"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
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
	ningboDevices   sync.Map
	hangzhouDevices sync.Map
	onlineDevices   map[string]*sync.Map
	template        = "2006-01-02 15:04:00"
)

func main() {
	onlineDevices = make(map[string]*sync.Map)
	onlineDevices["hangzhou"] = &hangzhouDevices
	onlineDevices["ningbo"] = &ningboDevices

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
	var (
		preTime      = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
		prevRecords  = make(map[string]int)
		firstRequest = true
	)
	// 定时任务
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
			log.Printf("Failed to get record at %s: %v", startTimeStr, err)
			break
		}

		for siteID, currInstances := range currRecords {
			log.Println("**********************************")
			var loginFailures int
			if firstRequest {
				// 首次需要特殊处理，直接登录相应数量用户
				loginFailures = deviceLogin("huadong", siteID, currInstances)
				if loginFailures != 0 {
					log.Printf("Failure: %d devices in %s failed to login", loginFailures, siteID)
				}
			} else {
				// 余下只需要跟上一分钟对比
				prevInstances, ok := prevRecords[siteID]
				if !ok {
					log.Printf("No previous record found for site %s", siteID)
					continue
				}
				diff := currInstances - prevInstances
				if diff > 0 {
					loginFailures = deviceLogin("huadong", siteID, diff)
					if loginFailures != 0 {
						log.Printf("Failure: %d devices in %s failed to login", loginFailures, siteID)
					}
				} else if diff < 0 {
					deviceLogout(-diff, "huadong", siteID)
				}
			}

			dbservice.InsertRecord("huadong", siteID, curTime.Format(template), deviceCount(siteID), loginFailures)
			log.Printf("Current: %d devices in %s are online now", deviceCount(siteID), siteID)

			prevRecords[siteID] = deviceCount(siteID)
		}
		preTime = curTime
		firstRequest = false

		log.Println()
		log.Println()
	}
}

// Returns: the number of login failure
func deviceLogin(zoneID string, siteID string, num int) int {
	var (
		wg      sync.WaitGroup
		devices []string
		mutex   sync.Mutex
	)

	for i := 0; i < num; i++ {
		wg.Add(1)
		// 异步调用登录接口
		go func() {
			defer wg.Done()
			var device Device
			// 获取一个不重复device_id
			for {
				randomId := uuid.NewUUID()
				deviceID := fmt.Sprintf("Dev-%s", randomId)
				if _, exist := onlineDevices[siteID].Load(deviceID); !exist {
					device = Device{ZoneID: zoneID, SiteID: siteID, DeviceID: deviceID}
					break
				}
			}

			// 向用户交互模块发出登录请求，获取实例信息
			instance, err := sendLoginRequest(device.DeviceID, device.ZoneID, device.SiteID)
			if err != nil {
				log.Printf("Failed to log in: %v", err)
				return
			} else if instance == nil {
				log.Println("The value of instance is nil")
				return
			}

			// 连接具体实例
			err = sendConnectRequest(device.DeviceID, instance.ServerIP, instance.Port)
			if err != nil {
				log.Printf("Failed to connect instance: %v", err)
				return
			}

			device.Host = instance.ServerIP
			device.Port = instance.Port

			mutex.Lock()
			onlineDevices[siteID].Store(device.DeviceID, device)
			devices = append(devices, device.DeviceID)
			mutex.Unlock()
		}()
	}

	wg.Wait()

	arrays := devices
	if len(arrays) > 3 {
		arrays = arrays[:3]
		arrays = append(arrays, "...")
	}
	log.Printf("[%s] %d devices logged in %s, %s", strings.Join(arrays, ", "), len(devices), siteID, zoneID)
	return num - len(devices)
}

func deviceLogout(num int, zoneID string, siteID string) {

	// 1. 选择num个在对应zone和site的设备
	var devices []string
	onlineDevices[siteID].Range(func(key, value interface{}) bool {
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
		value, _ := onlineDevices[siteID].Load(deviceID)
		device := value.(Device)

		onlineDevices[siteID].Delete(deviceID)

		ok := true
		// 2.1 向实例发出断开连接请求
		err := sendDisConnectRequest(device.Host, device.Port)
		if err != nil {
			log.Printf("Failed to disconnect instance: %v", err)
			ok = false
		}

		// 2.2 向用户交互模块发出登出请求
		err = sendLogoutRequest(device.DeviceID, device.ZoneID)
		if err != nil {
			log.Printf("Failed to log out from usercenter: %v", err)
			ok = false
		}

		if ok {
			devicesLoggedOut = append(devicesLoggedOut, deviceID)
		}
	}

	arrays := devicesLoggedOut
	if len(arrays) > 4 {
		arrays = arrays[:4]
		arrays = append(arrays, "...")
	}
	log.Printf("[%s] %d devices logged out in %s, %s", strings.Join(arrays, ", "), len(devicesLoggedOut), siteID, zoneID)
}

func deviceCount(siteID string) int {
	count := 0
	onlineDevices[siteID].Range(func(_, _ interface{}) bool {
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
		return fmt.Errorf("failed to send disconnect request: %w", err)
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
	urlStr := fmt.Sprintf("%s://%s:%s/%s", config.USERCENTERPROTOCOL, config.USERCENTERHOST, config.USERCENTERPORT, config.LOGINPATH)
	resp, err := http.PostForm(urlStr, url.Values{
		"zone_id":   {zoneId},
		"site_id":   {siteId},
		"device_id": {deviceId},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response Response
	err = json.Unmarshal([]byte(string(body)), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse respinse body: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to login with status code: %d, error message: %s", response.StatusCode, string(response.Data))
	}

	var deviceLoginResponse DeviceLoginResponse
	err = json.Unmarshal(response.Data, &deviceLoginResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response instance data: %w", err)
	}

	return deviceLoginResponse.Instance, nil
}

func sendLogoutRequest(deviceId string, zoneId string) error {
	urlStr := fmt.Sprintf("%s://%s:%s/%s", config.USERCENTERPROTOCOL, config.USERCENTERHOST, config.USERCENTERPORT, config.LOGOUTPATH)
	resp, err := http.PostForm(urlStr, url.Values{
		"zone_id":   {zoneId},
		"device_id": {deviceId},
	})
	if err != nil {
		return fmt.Errorf("failed to send logout request: %w", err)
	}
	defer resp.Body.Close()

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

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to logout with status code: %d, error message: %s", response.StatusCode, string(response.Data))
	}

	return nil
}
