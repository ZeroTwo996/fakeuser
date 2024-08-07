package service

import (
	"fakeuser/config"
	"fakeuser/database"
	"fmt"
)

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

// InsertRecord 插入记录到record表
func InsertRecord(zoneID string, siteID string, date string, instances int, loginFailures int) error {
	insertQuery := fmt.Sprintf("INSERT INTO record_%s (site_id, date, instances, login_failures) VALUES (?, ?, ?, ?)", zoneID)
	if _, err := database.DB.Exec(insertQuery, siteID, date, instances, loginFailures); err != nil {
		return err
	}

	return nil
}
