package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/fly-examples/postgres-ha/.flyd/scripts/util"
	"github.com/fly-examples/postgres-ha/pkg/privnet"
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		util.WriteError(err)
	}

	role := strings.Trim(string(body), "\n")
	role = strings.Trim(role, "\"")
	util.WriteOutput(role)
}
