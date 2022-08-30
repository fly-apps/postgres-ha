package util

import (
	"encoding/json"
	"fmt"
	"os"
)

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func WriteError(err error) {
	resp := &Response{
		Success: false,
		Message: err.Error(),
	}
	sendToStdout(resp)
}

func WriteOutput(message, data string) {
	resp := &Response{
		Success: true,
		Message: message,
		Data:    data,
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
