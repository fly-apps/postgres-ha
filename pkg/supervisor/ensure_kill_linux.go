// +build linux

package supervisor

import "syscall"

func ensureKill(cmd *exec.Cmd) {
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
