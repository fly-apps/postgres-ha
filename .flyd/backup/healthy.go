// package main

// import (
// 	"bytes"
// 	"flag"
// 	"fmt"
// 	"os"
// 	"os/exec"

// 	"github.com/fly-examples/postgres-ha/.flyd/scripts/util"
// 	"github.com/fly-examples/postgres-ha/pkg/privnet"
// )

// func main() {
// 	ipPtr := flag.String("ip", "", "Target internal ip address. Defaults to the internal ip of the Machine running script.")
// 	flag.Parse()

// 	if *ipPtr == "" {
// 		ip, err := privnet.PrivateIPv6()
// 		if err != nil {
// 			util.WriteError(err)
// 		}
// 		*ipPtr = ip.String()
// 	}

// 	util.SetEnvironment()

// 	subProcess := exec.Command("/usr/local/bin/stolonctl status")

// 	var outBuf, errBuf bytes.Buffer
// 	subProcess.Stdout = &outBuf
// 	subProcess.Stderr = &errBuf

// 	if err := subProcess.Run(); err != nil {
// 		panic(err)
// 	}

// 	// io.WriteString(stdin, *cmdPtr+"\n")
// 	// io.WriteString(stdin, "\\q"+"\n")

// 	// subProcess.Wait()

// 	if subProcess.ProcessState.ExitCode() != 0 {
// 		util.WriteError(fmt.Errorf(errBuf.String()))
// 		os.Exit(0)
// 	}

// 	util.WriteOutput(outBuf.String())
// 	os.Exit(0)

// }

// // export $(cat /data/.env | xargs)

// // IP=$(ip -6 addr show eth0 | grep -Eo '(fdaa.*)\/')
// // IP=${IP%?}

// // HEALTHY=$(stolonctl status | grep $IP | awk '{print $2}')

// // if [ $HEALTHY = "true" ] ; then
// //   echo '{"status": "success", "data":{"ok": true, "message": "'"$IP"' is healthy"}}'
// //   exit 0
// // fi

// // for i in $(seq 1 30)
// // do
// //   HEALTHY=$(stolonctl status | grep $IP | awk '{print $2}')
// //   if [ "$HEALTHY" = "true" ] ; then
// //     echo '{"status": "success", "data":{"ok": true, "message": "'"$IP"' is healthy"}}'
// //     exit 0
// //   fi
// //   sleep 1
// // done

// // echo '{"status": "failed", "data":{"ok": false, "error": "failed to become healthy within 30 seconds"}}'
// // exit 1
