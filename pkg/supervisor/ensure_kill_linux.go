// +build linux

package supervisor

import (
	"os/exec"
	"syscall"
)

func ensureKill(cmd *exec.Cmd) {
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
