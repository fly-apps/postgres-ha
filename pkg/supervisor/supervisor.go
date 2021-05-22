package supervisor

import (
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Supervisor struct {
	name        string
	output      *multiOutput
	procs       []*process
	procWg      sync.WaitGroup
	done        chan bool
	interrupted chan os.Signal
	timeout     time.Duration
}

func New(name string, timeout time.Duration) *Supervisor {
	return &Supervisor{
		timeout: 5 * time.Second,
		name:    "flypg",
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
	case <-h.interrupted:
	}
}

func (h *Supervisor) waitForTimeoutOrInterrupt() {
	select {
	case <-time.After(h.timeout):
	case <-h.interrupted:
	}
}

func (h *Supervisor) waitForExit() {
	h.waitForDoneOrInterrupt()

	for _, proc := range h.procs {
		go proc.Interrupt()
	}

	h.waitForTimeoutOrInterrupt()

	for _, proc := range h.procs {
		go proc.Kill()
	}
}

func (h *Supervisor) Run() {
	h.done = make(chan bool, len(h.procs))

	h.interrupted = make(chan os.Signal)
	signal.Notify(h.interrupted, syscall.SIGINT, syscall.SIGTERM)

	for _, proc := range h.procs {
		h.runProcess(proc)
	}

	go h.waitForExit()

	h.procWg.Wait()
}
