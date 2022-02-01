package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/fly-examples/postgres-ha/pkg/privnet"
	"github.com/fly-examples/postgres-ha/pkg/util"
)

func main() {
	ipPtr := flag.String("ip", "", "Target internal ip address. Defaults to the internal ip of the Machine running script.")
	flag.Parse()

	if *ipPtr == "" {
		ip, err := privnet.PrivateIPv6()
		if err != nil {
			util.WriteError(err)
		}
		*ipPtr = ip.String()
	}

	endpoint := fmt.Sprintf("http://[%s]:5500/flycheck/role", *ipPtr)
	resp, err := http.Get(endpoint)
	if err != nil {
		util.WriteError(err)
	}

	if resp.StatusCode != 200 {
		util.WriteError(fmt.Errorf("failed with status code: %v", resp.StatusCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		util.WriteError(err)
	}

	role := strings.Trim(string(body), "\n")
	role = strings.Trim(role, "\"")
	util.WriteOutput(role)
}
