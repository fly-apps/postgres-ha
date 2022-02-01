package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Response struct {
	Status string       `json:"status"`
	Data   ResponseData `json:"data"`
}

type ResponseData struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

func WriteError(err error) {
	resp := &Response{
		Status: "failed",
		Data: ResponseData{
			OK:    false,
			Error: err.Error(),
		},
	}
	sendToStdout(resp)
}

func WriteOutput(message string) {
	resp := &Response{
		Status: "success",
		Data: ResponseData{
			OK:      true,
			Message: message,
		},
	}
	sendToStdout(resp)
}

func sendToStdout(resp *Response) {
	e, err := json.Marshal(resp)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(e))
	os.Exit(0)
}

const EnvFile = ".env"

func SetEnvironment() error {
	file, err := os.Open(EnvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Println(os.Getenv("PATH"))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		arr := strings.Split(scanner.Text(), "=")
		fmt.Printf("Setting %s=%s\n", arr[0], arr[1])
		os.Setenv(arr[0], arr[1])
	}

	if scanner.Err(); err != nil {
		return err
	}

	return nil
}
