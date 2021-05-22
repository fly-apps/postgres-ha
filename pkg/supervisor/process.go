package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type process struct {
	*exec.Cmd
	name       string
	color      int
	output     *multiOutput
	stopSignal os.Signal
}

type Opt func(*process)

func WithEnv(env map[string]string) Opt {
	return func(proc *process) {
		for k, v := range env {
			proc.Env = append(proc.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
}

func WithStopSignal(sig os.Signal) Opt {
	return func(proc *process) {
		proc.stopSignal = sig
	}
}

func WithRootDir(dir string) Opt {
	return func(proc *process) {
		proc.Dir = dir
	}
}

func (p *process) writeLine(b []byte) {
	p.output.WriteLine(p, b)
}

func (p *process) writeErr(err error) {
	p.output.WriteErr(p, err)
}

func (p *process) signal(sig os.Signal) {
	group, err := os.FindProcess(-p.Process.Pid)
	if err != nil {
		p.writeErr(err)
		return
	}

	if err = group.Signal(sig); err != nil {
		p.writeErr(err)
	}
}

func (p *process) Running() bool {
	return p.Process != nil && p.ProcessState == nil
}

func (p *process) Run() {
	p.output.PipeOutput(p)
	defer p.output.ClosePipe(p)

	ensureKill(p)

	p.writeLine([]byte("\033[1mRunning...\033[0m"))

	if err := p.Cmd.Run(); err != nil {
		p.writeErr(err)
	} else {
		p.writeLine([]byte("\033[1mProcess exited\033[0m"))
	}
}

func (p *process) Interrupt() {
	if p.Running() {
		p.writeLine([]byte(fmt.Sprintf("\033[1mStopping %s...\033[0m", p.stopSignal)))
		p.signal(p.stopSignal)
	}
}

func (p *process) Kill() {
	if p.Running() {
		p.writeLine([]byte("\033[1mKilling...\033[0m"))
		p.signal(syscall.SIGKILL)
	}
}
