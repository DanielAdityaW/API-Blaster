package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	setupLogger()

	csvName := prompt("CSV file name (data/csv): ")
	jsonName := prompt("JSON template name (data/json): ")
	url := prompt("API URL: ")

	csvPath := filepath.Join("data", "csv", csvName)
	jsonPath := filepath.Join("data", "json", jsonName)

	template := readFile(jsonPath)
	records := readCSV(csvPath)

	var bodies []string

	for _, row := range records {
		body := fillTemplate(template, row)
		bodies = append(bodies, "[\n"+body+"\n]")
	}

	// preview
	preview := strings.Join(bodies, "\n\n")

	fileName := "preview_all.txt"
	err := os.WriteFile(fileName, []byte(preview), 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("\nPreview file created:", fileName)
	openFile(fileName)

	confirm := prompt("\nSend ALL requests? (y/n): ")

	if confirm != "y" {
		log.Println("Canceled.")
		return
	}

	log.Println("\nSending requests...\n")

	success := 0
	fail := 0

	for i, row := range records {

		log.Println("====================================")
		log.Println("Sending Row", i+1)
		log.Println("Time:", time.Now().Format("15:04:05"))

		filePath := resolvePath(row["pathFile"])
		body := fillTemplate(template, row)

		log.Println("File:", filePath)

		ok := sendPOST(url, body, filePath)

		if ok {
			log.Println("Result: SUCCESS")
			success++
		} else {
			log.Println("Result: FAILED")
			fail++
		}

		log.Println("Waiting 3 seconds...\n")
		time.Sleep(3 * time.Second)
	}

	log.Println("\n===== RESULT =====")
	log.Println("Success:", success)
	log.Println("Failed :", fail)
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func readCSV(path string) []map[string]string {

	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	headers, err := reader.Read()
	if err != nil {
		log.Fatal(err)
	}

	var rows []map[string]string

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		row := map[string]string{}

		for i, val := range record {
			row[headers[i]] = val
		}

		rows = append(rows, row)
	}

	return rows
}

func fillTemplate(template string, data map[string]string) string {
	result := template

	for k, v := range data {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}

	return result
}

func sendPOST(url string, body string, filePath string) bool {

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// JSON field
	if err := writer.WriteField("data", body); err != nil {
		log.Println(err)
		return false
	}

	// file part
	if filePath != "" {

		file, err := os.Open(filePath)
		if err != nil {
			log.Println("File error:", err)
			return false
		}

		part, err := writer.CreateFormFile("mediafile", filepath.Base(filePath))
		if err != nil {
			log.Println(err)
			file.Close()
			return false
		}

		if _, err := io.Copy(part, file); err != nil {
			log.Println(err)
			file.Close()
			return false
		}

		file.Close()
	}

	if err := writer.Close(); err != nil {
		log.Println(err)
		return false
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(requestBody.Bytes()))
	if err != nil {
		log.Println(err)
		return false
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Connection", "close")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			MaxIdleConns:      0,
			IdleConnTimeout:   1 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Request failed:", err)
		return false
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	log.Println("Status:", resp.Status)
	log.Println("Response:", string(respBody))

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func openFile(path string) {

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	cmd.Start()
}

func prompt(label string) string {
	log.Print(label)
	var input string
	fmt.Scanln(&input)
	return strings.TrimSpace(input)
}

func setupLogger() {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := "log_send_" + timestamp + ".txt"

	logDir := "logs"
	filePath := filepath.Join(logDir, fileName)

	err := os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		log.Fatal("Cannot create log directory:", err)
	}

	logFile, err := os.Create(filePath)
	if err != nil {
		log.Fatal("Cannot create log file:", err)
	}

	multi := io.MultiWriter(os.Stdout, logFile)

	log.SetOutput(multi)
	log.Println("Log file:", filePath)
}

func resolvePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
