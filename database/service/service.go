package service

import (
	"fakeuser/config"
	"fakeuser/database"
	"fmt"
	"strings"
)

// RecordData 用于存储要插入的记录数据
type RecordData struct {
	SiteID        string
	Date          string
	Instances     int
	LoginFailures int
}

// GetFirstRecord 获取history表中最早的日期
func GetFirstRecord() (string, error) {
	rows, err := database.DB.Query("SELECT date FROM history_huadong ORDER BY date LIMIT 1")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	if rows.Next() {
		var date string
		err := rows.Scan(&date)
		if err != nil {
			return "", err
		}
		return date, nil
	} else {
		return "", fmt.Errorf("no records found")
	}
}

// GetLastRecord 获取history表中最晚的日期
func GetLastRecord() (string, error) {
	rows, err := database.DB.Query("SELECT date FROM history_huadong ORDER BY date DESC LIMIT 1")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	if rows.Next() {
		var date string
		err := rows.Scan(&date)
		if err != nil {
			return "", err
		}
		return date, nil
	} else {
		return "", fmt.Errorf("no records found")
	}
}

// GetRecordWithDate 获取history表中指定date的记录
func GetRecordWithDate(date string) (map[string]int, error) {
	rows, err := database.DB.Query("SELECT site_id, instances FROM history_huadong WHERE date = ?", date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make(map[string]int)
	fmt.Printf("%s:", date)
	for rows.Next() {
		var site_id string
		var instances int
		err := rows.Scan(&site_id, &instances)
		if err != nil {
			return nil, err
		}
		records[site_id] = instances / config.SCALERATIO
		fmt.Printf("	%s: %d", site_id, records[site_id])
	}
	fmt.Println()
	return records, nil
}

// InsertRecords 插入记录到record表
func InsertRecords(zoneID string, records []RecordData) error {
	// 构建SQL语句
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("INSERT INTO record_%s (site_id, date, instances, login_failures) VALUES ", zoneID))

	// 动态添加每条记录
	first := true
	for _, record := range records {
		if !first {
			builder.WriteString(", ")
		}
		builder.WriteString("(")
		builder.WriteString("'")
		builder.WriteString(record.SiteID)
		builder.WriteString("', '")
		builder.WriteString(record.Date)
		builder.WriteString("', ")
		builder.WriteString(fmt.Sprintf("%d", record.Instances))
		builder.WriteString(", ")
		builder.WriteString(fmt.Sprintf("%d", record.LoginFailures))
		builder.WriteString(")")
		first = false
	}

	insertQuery := builder.String()
	if _, err := database.DB.Exec(insertQuery); err != nil {
		return err
	}

	return nil
}
