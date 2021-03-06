package util

import (
	"fmt"
	"os/exec"
)

// RunCmd runs specified command with arguments and returns its standard output.
func RunCmd(cmdName string, cmdArgs ...string) (string, error) {
	out, err := exec.Command(cmdName, cmdArgs...).Output() // nolint: gas
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("exit error (%s): %s", exitErr.Error(), exitErr.Stderr)
		}
		return "", err
	}

	return string(out), nil
}
