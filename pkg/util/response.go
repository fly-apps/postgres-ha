package util

import (
	"encoding/json"
	"fmt"
	"os"
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
