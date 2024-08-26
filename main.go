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
	httpClient      *http.Client
)

func main() {
	httpClient = &http.Client{
		Timeout: 2 * time.Second,
	}

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

		var zoneRecords []dbservice.RecordData
		for siteID, currInstances := range currRecords {
			log.Println("**********************************")
			log.Printf("Dealing '%s' site...\n", siteID)
			var loginFailures int
			if firstRequest {
				// 首次需要特殊处理，直接登录相应数量用户
				loginFailures = deviceLogin("huadong", siteID, currInstances)
				if loginFailures != 0 {
					log.Printf("Warning: %d devices failed to login", loginFailures)
				}
			} else {
				// 余下只需要跟上一分钟对比
				prevInstances, ok := prevRecords[siteID]
				if !ok {
					log.Printf("No previous record found")
					continue
				}
				diff := currInstances - prevInstances
				if diff > 0 {
					loginFailures = deviceLogin("huadong", siteID, diff)
					if loginFailures != 0 {
						log.Printf("Warning: %d devices failed to login", loginFailures)
					}
				} else if diff < 0 {
					logoutDevices(-diff, siteID)
				}
			}

			zoneRecords = append(zoneRecords, dbservice.RecordData{SiteID: siteID, Date: curTime.Format(template), Instances: deviceCount(siteID), LoginFailures: loginFailures})
			log.Printf("Current: %d devices are online now", deviceCount(siteID))

			prevRecords[siteID] = deviceCount(siteID)
		}
		if config.RECORDENABLED {
			err := dbservice.InsertRecords("huadong", zoneRecords)
			if err != nil {
				log.Printf("Failed to insert records: %v", err)
			}
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
			instance, err := sendLoginRequest(device.ZoneID, device.SiteID, device.DeviceID)
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
	log.Printf("[%s] %d devices logged in", strings.Join(arrays, ", "), len(devices))
	return num - len(devices)
}

func logoutDevice(device Device) bool {
	// 1. 向实例发出断开连接请求
	err := sendDisConnectRequest(device.Host, device.Port)
	if err != nil {
		log.Printf("Failed to disconnect instance: %v", err)
		return false
	}
	// 2. 断开连接成功后，向用户交互模块发出登出请求
	err = sendLogoutRequest(device.ZoneID, device.SiteID, device.DeviceID)
	if err != nil {
		log.Printf("Failed to log out from usercenter: %v", err)
		return false
	}

	return true
}

func logoutDevices(num int, siteID string) {
	// 1. 筛选绑定实例可用（网络可达）的在线设备
	var wg sync.WaitGroup
	var instanceHealthyDevices sync.Map
	onlineDevices[siteID].Range(func(key, value any) bool {
		device := value.(Device)
		wg.Add(1)
		go func(d Device) {
			defer wg.Done()

			if healthy := checkInstanceHealthy(d); healthy {
				instanceHealthyDevices.Store(d.DeviceID, d)
			}
		}(device)
		return true
	})
	wg.Wait()

	// 2. 登出num个设备
	var devicesLoggedOut []string
	var attemptCount = 0
	var mu sync.Mutex
	wg = sync.WaitGroup{}
	instanceHealthyDevices.Range(func(key, value any) bool {
		device := value.(Device)
		if attemptCount == num {
			return false
		} else {
			attemptCount++
		}
		wg.Add(1)
		go func(d Device) {
			defer wg.Done()

			if ok := logoutDevice(d); ok {
				mu.Lock()
				devicesLoggedOut = append(devicesLoggedOut, d.DeviceID)
				mu.Unlock()
			}
			onlineDevices[siteID].Delete(d.DeviceID) // 登出失败也删除

		}(device)

		return true
	})
	wg.Wait()

	// 3. 日志打印，判断是否num个数量成功登出
	successCount := len(devicesLoggedOut)
	printArray := devicesLoggedOut
	if successCount > 4 {
		printArray = devicesLoggedOut[:3]
		printArray = append(printArray, "...")
		printArray = append(printArray, devicesLoggedOut[len(devicesLoggedOut)-1])
	}
	log.Printf("[%s] %d devices logged out in", strings.Join(printArray, ", "), len(devicesLoggedOut))

	if successCount != num {
		log.Printf("Error: Trying to log out %d devices, but only %d devices logged out", num, successCount)
	}
}

func deviceCount(siteID string) int {
	count := 0
	onlineDevices[siteID].Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func checkInstanceHealthy(device Device) bool {
	url := fmt.Sprintf("http://%s:%d/healthz", device.Host, device.Port)

	resp, err := httpClient.Get(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		return true
	}
	return false
}

func sendConnectRequest(deviceId string, host string, port int) error {
	urlStr := fmt.Sprintf("http://%s:%d/connect", host, port)
	for i := 0; i < 2; i++ {
		resp, err := httpClient.PostForm(urlStr, url.Values{
			"device_id": {deviceId},
		})
		if err != nil {
			log.Printf("Connect attempt %d failed when requesting: %v", i+1, err)
			time.Sleep(1 * time.Second)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		log.Printf("Connect attempt %d failed with status code: %d", i+1, resp.StatusCode)
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("error with connect request after retries")
}

func sendDisConnectRequest(host string, port int) error {
	urlStr := fmt.Sprintf("http://%s:%d/disconnect", host, port)

	for i := 0; i < 2; i++ {
		resp, err := httpClient.Get(urlStr)
		if err != nil {
			log.Printf("Disconnect attempt %d failed when requesting: %v", i+1, err)
			time.Sleep(1 * time.Second)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		log.Printf("Disconnect attempt %d failed with status code: %d", i+1, resp.StatusCode)
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("error with disconnect request after retries")
}

func sendLoginRequest(zoneId string, siteId string, deviceId string) (*Instance, error) {
	urlStr := fmt.Sprintf("%s://%s:%s/%s", config.USERCENTERPROTOCOL, config.USERCENTERHOST, config.USERCENTERPORT, config.LOGINPATH)
	resp, err := httpClient.PostForm(urlStr, url.Values{
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

func sendLogoutRequest(zoneId string, siteId string, deviceId string) error {
	urlStr := fmt.Sprintf("%s://%s:%s/%s", config.USERCENTERPROTOCOL, config.USERCENTERHOST, config.USERCENTERPORT, config.LOGOUTPATH)
	resp, err := httpClient.PostForm(urlStr, url.Values{
		"zone_id":   {zoneId},
		"site_id":   {siteId},
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
