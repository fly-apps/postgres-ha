package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/fly-apps/postgres-ha/pkg/server"
	"github.com/google/shlex"
	"golang.org/x/sync/errgroup"
)

type processError struct {
	process *process
}

func (pe *processError) Error() string {
	return fmt.Sprintf("process %s failed", pe.process.name)
}

type Supervisor struct {
	name    string
	output  *multiOutput
	procs   []*process
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
		cmd.Env = proc.env
		cmd.Dir = proc.dir

		return cmd
	}

	for _, opt := range opts {
		opt(proc)
	}

	proc.output.Connect(proc)

	h.procs = append(h.procs, proc)
}

func (h *Supervisor) runProcess(ctx context.Context, proc *process) error {
	restarts := 0

	for {
		proc.Run()

		// supervisor is stopping, exit
		if ctx.Err() != nil {
			return nil
		}

		// process is done, exit
		if !proc.restart {
			proc.writeLine([]byte("done"))
			return nil
		}

		// process restart limit reached, crash supervisor
		if proc.maxRestarts > 0 && restarts >= proc.maxRestarts {
			proc.writeLine([]byte("restart attempts exhausted, crashing"))
			return &processError{proc}
		}

		restarts++
		proc.writeLine([]byte(fmt.Sprintf("restarting in %s [attempt %d]", proc.restartDelay, restarts)))
		select {
		case <-time.After(proc.restartDelay):
		case <-ctx.Done():
			return nil
		}
	}
}

func (h *Supervisor) waitForTimeoutOrInterrupt() {
	select {
	case <-time.After(h.timeout):
	case <-h.stop:
	}
}

func (h *Supervisor) waitForExit(ctx context.Context) {
	<-ctx.Done()

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
	go server.StartHttpServer()
}

func (h *Supervisor) Run() error {
	h.stop = make(chan struct{})

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-h.stop
		cancel()
	}()

	eg, egCtx := errgroup.WithContext(ctx)

	for _, proc := range h.procs {
		p := proc
		eg.Go(func() error {
			return h.runProcess(egCtx, p)
		})
	}

	go h.waitForExit(egCtx)

	return eg.Wait()
}

func (h *Supervisor) Stop() {
	h.stop <- struct{}{}
}

func (h *Supervisor) StopOnSignal(sigs ...os.Signal) {
	sigch := make(chan os.Signal)
	signal.Notify(sigch, sigs...)

	go func() {
		for sig := range sigch {
			fmt.Printf("Got %s, stopping\n", sig)
			h.Stop()
		}
	}()
}
