package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flycheck"
)

type Supervisor struct {
	name    string
	output  *multiOutput
	procs   []*process
	procWg  sync.WaitGroup
	done    chan bool
	stop    chan struct{}
	timeout time.Duration
}

func New(name string, timeout time.Duration) *Supervisor {
	return &Supervisor{
		timeout: 5 * time.Second,
		name:    name,
		output:  &multiOutput{},
	}
}

var colors = []int{2, 3, 4, 5, 6, 42, 130, 103, 129, 108}

func (h *Supervisor) AddProcess(name string, command string, opts ...Opt) {
	cmd := exec.Command("/bin/sh", "-c", "gosu stolon "+command)
	// cmd.SysProcAttr = &syscall.SysProcAttr{}
	// cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid}

	proc := &process{
		name:       name,
		Cmd:        cmd,
		color:      colors[len(h.procs)%len(colors)],
		output:     h.output,
		stopSignal: syscall.SIGINT,
	}

	proc.Env = os.Environ()

	for _, opt := range opts {
		opt(proc)
	}

	proc.output.Connect(proc)

	h.procs = append(h.procs, proc)
}

func (h *Supervisor) runProcess(proc *process) {
	h.procWg.Add(1)

	go func() {
		defer h.procWg.Done()
		defer func() { h.done <- true }()

		proc.Run()
	}()
}

func (h *Supervisor) waitForDoneOrInterrupt() {
	select {
	case <-h.done:
	case <-h.stop:
	}
}

func (h *Supervisor) waitForTimeoutOrInterrupt() {
	select {
	case <-time.After(h.timeout):
	case <-h.stop:
	}
}

func (h *Supervisor) waitForExit() {
	h.waitForDoneOrInterrupt()

	fmt.Println("supervisor stopping")

	for _, proc := range h.procs {
		go proc.Interrupt()
	}

	h.waitForTimeoutOrInterrupt()

	for _, proc := range h.procs {
		go proc.Kill()
	}
}

func (h *Supervisor) StartHttpListener() {
	flycheck.StartCheckListener()
}

func (h *Supervisor) Run() {
	h.done = make(chan bool, len(h.procs))
	h.stop = make(chan struct{})

	for _, proc := range h.procs {
		h.runProcess(proc)
	}

	go h.waitForExit()

	h.procWg.Wait()
}

func (h *Supervisor) Stop() {
	h.stop <- struct{}{}
}
