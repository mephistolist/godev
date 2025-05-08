package client

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

func CheckRsync(user, password, host string, port int, timeout time.Duration) error {
	_, err := RunCommand(user, password, host, port, timeout, "true") // dummy command
	if err != nil {
		return err
	}

	cmd := "which rsync"
	out, err := RunCommand(user, password, host, port, timeout, cmd)
	if err != nil || out == "" {
		return fmt.Errorf("rsync not found on host %s", host)
	}

	return nil
}

func RsyncScript(user, password, host string, port int, localPath string) error {
	cmd := exec.Command("rsync", "-e", fmt.Sprintf("ssh -p %d", port), localPath, fmt.Sprintf("%s@%s:/tmp/", user, host))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("rsync error: %s", stderr.String())
	}
	return nil
}

func RunRemoteScript(user, password, host string, port int, timeout time.Duration, scriptPath string) (string, error) {
	scriptName := filepath.Base(scriptPath)
	remoteScript := "/tmp/" + scriptName

	// 1. Rsync to remote /tmp/
	err := RsyncScript(user, password, host, port, scriptPath)
	if err != nil {
		return "", err
	}

	// 2. chmod +x and run
	cmd := fmt.Sprintf("chmod +x %s && %s", remoteScript, remoteScript)
	return RunCommand(user, password, host, port, timeout, cmd)
}

// New function: Run script using sudo
func RunRemoteScriptWithSudo(user, sshPassword, sudoPassword, host string, port int, timeout time.Duration, scriptPath string) (string, error) {
	scriptName := filepath.Base(scriptPath)
	remoteScript := "/tmp/" + scriptName

	// 1. Rsync script to remote /tmp/
	err := RsyncScript(user, sshPassword, host, port, scriptPath)
	if err != nil {
		return "", err
	}

	// 2. chmod +x the script
	chmodCmd := fmt.Sprintf("chmod +x %s", remoteScript)
	if _, err := RunCommand(user, sshPassword, host, port, timeout, chmodCmd); err != nil {
		return "", fmt.Errorf("chmod failed: %v", err)
	}

	// 3. Run with sudo if password provided
	var cmd string
	if sudoPassword != "" {
		cmd = fmt.Sprintf("echo %q | sudo -S %s", sudoPassword, remoteScript)
	} else {
		cmd = remoteScript // fallback to non-sudo
	}

	return RunCommand(user, sshPassword, host, port, timeout, cmd)
}

// SSH command execution
func RunCommand(user, password, host string, port int, timeout time.Duration, command string) (string, error) {
	conn, session, err := connectSSH(user, password, host, port, timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	defer session.Close()

	var outputBuf, stderrBuf bytes.Buffer
	session.Stdout = &outputBuf
	session.Stderr = &stderrBuf

	err = session.Run(command)
	if err != nil {
		return "", fmt.Errorf("ssh command error: %v\nstderr: %s", err, stderrBuf.String())
	}

	return outputBuf.String(), nil
}
