package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Response struct {
	Response struct {
		ComMsgHeader string `json:"comMsgHeader"`
		MsgHeader    struct {
			QueryTime     string `json:"queryTime"`
			ResultCode    int    `json:"resultCode"`
			ResultMessage string `json:"resultMessage"`
		} `json:"msgHeader"`
		MsgBody struct {
			BusArrivalList []BusArrival `json:"busArrivalList"`
		} `json:"msgBody"`
	} `json:"response"`
}

type BusArrival struct {
	RouteName      interface{} `json:"routeName"`
	StationNm1     string      `json:"stationNm1"`
	PredictTime1   interface{} `json:"predictTime1"`
	RemainSeatCnt1 interface{} `json:"remainSeatCnt1"`
}

func main() {
	if err := runBatch(time.Now()); err != nil {
		fmt.Printf("Initial batch failed: %v\n", err)
	}

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case t := <-ticker.C:
			if err := runBatch(t); err != nil {
				fmt.Printf("Batch failed at %v: %v\n", t, err)
			}
		}
	}
}

func runBatch(t time.Time) error {
	fmt.Printf("Running batch at %v\n", t)
	apiURL := "https://apis.data.go.kr/6410000/busarrivalservice/v2/getBusArrivalListv2"
	serviceKey := "eBy82LnzKjjcARxOgTGpnyT6lV6B0rO%2FyzLDR6D9SiY0LSegVWLZI7KjduZFCa7g8rXOTzNtssYcH9Xz8gWS8Q%3D%3D"
	stationID := "226000039" // 의왕톨게이트 (잠실행)

	url := fmt.Sprintf("%s?serviceKey=%s&stationId=%s&format=json", apiURL, serviceKey, stationID)

	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResponse Response
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Filter and check conditions
	filteredData := [][]string{{"time", "predict_time", "remain_seat", "station_name"}}
	for _, bus := range apiResponse.Response.MsgBody.BusArrivalList {
		routeNameInt, err := ConvertToInt(bus.RouteName)
		if err != nil {
			continue
		}

		if routeNameInt != 1009 {
			continue
		}

		predictTime, err := ConvertToInt(bus.PredictTime1)
		if err != nil || predictTime >= 10 {
			continue
		}

		remainSeats, err := ConvertToInt(bus.RemainSeatCnt1)
		if err != nil {
			remainSeats = -1 // Handle missing data
		}

		// Append to filtered data as a row
		filteredData = append(filteredData, []string{
			t.Format("2006-01-02 15:04:05"),
			strconv.Itoa(predictTime),
			strconv.Itoa(remainSeats),
			bus.StationNm1,
		})
	}

	// Save to CSV if there's filtered data
	if len(filteredData) > 1 { // Header + at least one row
		if err := saveToCSVFile("./data.csv", filteredData); err != nil {
			return fmt.Errorf("failed to save CSV file: %w", err)
		}
	}

	return nil
}

// saveToCSVFile writes filtered data to a CSV file.
func saveToCSVFile(filename string, data [][]string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	for _, row := range data {
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// ConvertToInt converts an interface{} to an int.
func ConvertToInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case string:
		return strconv.Atoi(v)
	case int:
		return v, nil
	case float64: // If JSON numbers are parsed as float64
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected type %T", v)
	}
}
