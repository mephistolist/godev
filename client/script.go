package client

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Checks if rsync is available on the remote host
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

// Rsync a file to the remote host
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

// Run a remote script (Unix-like hosts)
func RunRemoteScript(user, password, host string, port int, timeout time.Duration, scriptPath string) (string, error) {
	scriptName := filepath.Base(scriptPath)
	remoteScript := "/tmp/" + scriptName

	if err := RsyncScript(user, password, host, port, scriptPath); err != nil {
		return "", err
	}

	cmd := fmt.Sprintf("chmod +x %s && %s", remoteScript, remoteScript)
	return RunCommand(user, password, host, port, timeout, cmd)
}

// Run remote script with sudo
func RunRemoteScriptWithSudo(user, sshPassword, sudoPassword, host string, port int, timeout time.Duration, scriptPath string) (string, error) {
	scriptName := filepath.Base(scriptPath)
	remoteScript := "/tmp/" + scriptName

	if err := RsyncScript(user, sshPassword, host, port, scriptPath); err != nil {
		return "", err
	}

	chmodCmd := fmt.Sprintf("chmod +x %s", remoteScript)
	if _, err := RunCommand(user, sshPassword, host, port, timeout, chmodCmd); err != nil {
		return "", fmt.Errorf("chmod failed: %v", err)
	}

	var cmd string
	if sudoPassword != "" {
		cmd = fmt.Sprintf("echo %q | sudo -S %s", sudoPassword, remoteScript)
	} else {
		cmd = remoteScript
	}

	return RunCommand(user, sshPassword, host, port, timeout, cmd)
}

// Windows-specific remote script execution (integrated from WinSync)
func RunWindowsRemoteScript(user, password, host string, port int, timeout time.Duration, scriptPath string) (string, error) {
	scriptName := filepath.Base(scriptPath)
	remotePath := "C:\\tmp\\" + scriptName

	// Create remote temp dir
	mkTmpCmd := `powershell -Command "if (!(Test-Path C:\\tmp)) { New-Item -ItemType Directory -Path C:\\tmp }"`
	if _, err := RunCommand(user, password, host, port, timeout, mkTmpCmd); err != nil {
		return "", fmt.Errorf("failed to create C:\\tmp: %v", err)
	}

	if err := SFTPUpload(user, password, host, port, timeout, scriptPath, remotePath); err != nil {
		return "", fmt.Errorf("sftp upload failed: %v", err)
	}

	// Execute script using cmd.exe
	execCmd := fmt.Sprintf(`cmd /C "C:\\tmp\\%s"`, scriptName)
	return RunCommand(user, password, host, port, timeout, execCmd)
}

// Generic SSH command runner
func RunCommand(user, password, host string, port int, timeout time.Duration, command string) (string, error) {
	conn, session, err := connectSSH(user, password, host, port, timeout) // <- Uses run.go version
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

// Upload file using SFTP (used by Windows execution)
func SFTPUpload(user, password, host string, port int, timeout time.Duration, localPath, remotePath string) error {
	client, err := connectSSHRaw(user, password, host, port, timeout)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to start sftp subsystem: %v", err)
	}
	defer sftpClient.Close()

	dstFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("cannot create remote file: %v", err)
	}
	defer dstFile.Close()

	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("cannot open local file: %v", err)
	}
	defer srcFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy failed: %v", err)
	}
	return nil
}

// SSH connection helper (client only)
func connectSSHRaw(user, password, host string, port int, timeout time.Duration) (*ssh.Client, error) {
	var methods []ssh.AuthMethod

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			agentClient := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	if strings.TrimSpace(password) != "" {
		methods = append(methods, ssh.Password(password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no authentication methods available (no password or SSH agent)")
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	return ssh.Dial("tcp", addr, config)
}
