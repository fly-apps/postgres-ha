package main

import (
	"fmt"
	"os"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/supervisor"
)

func main() {
	s := supervisor.New("supervisor", 10*time.Second)

	s.AddProcess("howdy", "ruby ./testscripts/say.rb howdy", supervisor.WithRestart(3, 2*time.Second))
	s.AddProcess("hello", "ruby ./testscripts/say.rb hello", supervisor.WithRestart(3, 2*time.Second))
	s.AddProcess("crash-limit", "ruby ./testscripts/crash.rb", supervisor.WithRestart(3, 1*time.Second))
	s.AddProcess("slow-stop", "ruby ./testscripts/slowstop.rb")
	s.AddProcess("crash-forever", "ruby ./testscripts/crash.rb", supervisor.WithRestart(0, 2*time.Second))
	s.AddProcess("onetime", "ruby ./testscripts/crash.rb")

	// ctx := supervisor.NewCancellableContext(syscall.SIGINT, syscall.SIGTERM)

	s.StopOnSignal(os.Interrupt)

	fmt.Println("starting")
	err := s.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
