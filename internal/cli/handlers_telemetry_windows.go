//go:build windows

package cli

import (
	"os/exec"
)

func configureDaemonSysProcAttr(cmd *exec.Cmd) {
	// On Windows, syscall.SysProcAttr has no Setsid field.
}
