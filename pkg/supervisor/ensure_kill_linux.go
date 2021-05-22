// +build linux

package supervisor

import "syscall"

func ensureKill(p *process) {
	p.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
