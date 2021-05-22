// +build !linux

package supervisor

func ensureKill(p *process) {
	// p.SysProcAttr.Pdeathsig in supported on on Linux, we can't do anything here
}
