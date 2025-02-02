package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
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
	RouteName      any    `json:"routeName"`
	StationNm1     string `json:"stationNm1"`
	PredictTime1   any    `json:"predictTime1"`
	RemainSeatCnt1 any    `json:"remainSeatCnt1"`
}

func main() {

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	serviceKey := os.Getenv("SERVICE_KEY")
	if err := runBatch(time.Now(), serviceKey); err != nil {
		fmt.Printf("Initial batch failed: %v\n", err)
		return
	}

	//run every 10 minutes
	//ticker := time.NewTicker(10 * time.Minute)
	//defer ticker.Stop()
	//
	//for {
	//	select {
	//	case t := <-ticker.C:
	//		if err := runBatch(t, serviceKey); err != nil {
	//			fmt.Printf("Batch failed at %v: %v\n", t, err)
	//		}
	//	}
	//}
}

func runBatch(t time.Time, serviceKey string) error {
	fmt.Printf("Running batch at %v\n", t)
	apiURL := "https://apis.data.go.kr/6410000/busarrivalservice/v2/getBusArrivalListv2"
	stationID := "226000039" // 의왕톨게이트 (잠실행)

	url := fmt.Sprintf("%s?serviceKey=%s&stationId=%s&format=json", apiURL, serviceKey, stationID)
	// 요청 수행
	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing response body")
		}
	}(response.Body)

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResponse Response
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Filter and check conditions
	filteredData := [][]string{{}}
	for _, bus := range apiResponse.Response.MsgBody.BusArrivalList {
		routeNameInt, err := ConvertToInt(bus.RouteName)
		if err != nil {
			continue
		}

		if routeNameInt != 1009 {
			//	continue
		}
		//
		predictTime, err := ConvertToInt(bus.PredictTime1)
		//if err != nil || predictTime >= 10 {
		//	continue
		//}
		//
		remainSeats, err := ConvertToInt(bus.RemainSeatCnt1)
		//if err != nil {
		//	remainSeats = -1 // Handle missing data
		//}

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

	return logResult(t, len(filteredData), nil)
}

// logResult writes the batch result to a log file.
func logResult(t time.Time, count int, err error) error {
	logFile := "./result.log"
	f, fileErr := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		fmt.Printf("Failed to open log file: %v\n", fileErr)
		return fileErr
	}
	defer f.Close()

	status := "success"
	if err != nil {
		status = fmt.Sprintf("error: %v", err)
	}

	logEntry := fmt.Sprintf(
		"%s | Count: %d | Status: %s\n",
		t.Format("2006-01-02 15:04:05"),
		count,
		status,
	)

	if _, writeErr := f.WriteString(logEntry); writeErr != nil {
		fmt.Printf("Failed to write to log file: %v\n", writeErr)
		return writeErr
	}

	return err
}

func saveToCSVFile(filename string, data [][]string) error {
	// Check if the file exists
	fileExists := false
	fileInfo, err := os.Stat(filename)
	if err == nil && fileInfo.Size() > 0 {
		fileExists = true
	}

	// Open file in append mode
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Write header only if the file is new/empty
	if !fileExists {
		if err := writer.Write([]string{"time", "predict_time", "remain_seat", "station_name"}); err != nil {
			return err
		}
	}

	// Write the data rows (skip header if present in data)
	for _, row := range data[1:] { // Skip header row if data contains it
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// ConvertToInt converts an any to an int.
func ConvertToInt(value any) (int, error) {
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
