// +build !linux

package supervisor

import "os/exec"

func ensureKill(cmd *exec.Cmd) {
	// cmd.SysProcAttr.Pdeathsig in supported on on Linux, we can't do anything here
}
