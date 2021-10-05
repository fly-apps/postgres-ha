package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/fly-examples/postgres-ha/pkg/flycheck"
	"github.com/google/shlex"
)

type Supervisor struct {
	name     string
	output   *multiOutput
	procs    []*process
	procWg   sync.WaitGroup
	crash    chan struct{}
	stop     chan struct{}
	stopping bool
	timeout  time.Duration
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
	proc := &process{
		name:       name,
		color:      colors[len(h.procs)%len(colors)],
		output:     h.output,
		stopSignal: syscall.SIGINT,
		env:        os.Environ(),
	}

	parsedCmd, err := shlex.Split(command)
	fatalOnErr(err)

	proc.f = func() *exec.Cmd {
		cmd := exec.Command(parsedCmd[0], parsedCmd[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		// cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid}

		cmd.Env = proc.env

		return cmd
	}

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

		restarts := 0

		for {
			proc.Run()

			// supervisor is stopping, exit
			if h.stopping {
				break
			}

			// process is done, exit
			if !proc.restart {
				proc.writeLine([]byte("done"))
				break
			}

			// process restart limit reached, crash supervisor
			if proc.maxRestarts > 0 && restarts >= proc.maxRestarts {
				proc.writeLine([]byte("restart attempts exhausted, crashing"))
				h.crash <- struct{}{}
				break
			}

			restarts++
			if proc.restartDelay > 0 {
				proc.writeLine([]byte(fmt.Sprintf("restarting in %s", proc.restartDelay)))
				time.Sleep(proc.restartDelay)
			}

			proc.writeLine([]byte(fmt.Sprintf("restarting [attempt %d]", restarts)))
		}
	}()
}

func (h *Supervisor) waitForCrashOrInterrupt() {
	select {
	case <-h.crash:
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
	h.waitForCrashOrInterrupt()

	fmt.Println("supervisor stopping")
	h.stopping = true

	for _, proc := range h.procs {
		go proc.Interrupt()
	}

	h.waitForTimeoutOrInterrupt()

	for _, proc := range h.procs {
		go proc.Kill()
	}
}

func (h *Supervisor) StartHttpListener() {
	go flycheck.StartCheckListener()
}

func (h *Supervisor) Run() {
	h.crash = make(chan struct{}, len(h.procs))
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

func (h *Supervisor) Wait() {
	h.procWg.Wait()
}
