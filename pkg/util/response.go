package util

import (
	"encoding/json"
	"fmt"
	"os"
)

type Response struct {
	OK     bool         `json:"ok"`
	Status string       `json:"status"`
	Data   ResponseData `json:"data"`
}

type ResponseData struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func WriteError(err error) {
	resp := &Response{
		Status: "failed",
		OK:     false,
		Data: ResponseData{
			Error: err.Error(),
		},
	}
	sendToStdout(resp)
}

func WriteOutput(message string) {
	resp := &Response{
		Status: "success",
		OK:     true,
		Data: ResponseData{
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
