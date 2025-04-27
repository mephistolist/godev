package client

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	skeemakh "github.com/skeema/knownhosts"
)

func Run(user, password, filePath, host string, port int, timeout time.Duration) (string, error) {
	// Read all commands from file into a single big script
	var script string
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			script += line + "\n"
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scanner error: %w", err)
	}

	// Establish SSH connection once
	conn, session, err := connectSSH(user, password, host, port, timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	defer session.Close()

	var outputBuf, stderrBuf bytes.Buffer
	session.Stdout = &outputBuf
	session.Stderr = &stderrBuf

	// Run the big script
	err = session.Run(script)
	if err != nil {
		return "", fmt.Errorf("ssh command error: %v\nstderr: %s", err, stderrBuf.String())
	}

	return outputBuf.String(), nil
}

func connectSSH(user, password, host string, port int, timeout time.Duration) (*ssh.Client, *ssh.Session, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("get home directory: %w", err)
	}

	khPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	kh, err := skeemakh.NewDB(khPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load known_hosts DB: %w", err)
	}

	var authMethods []ssh.AuthMethod
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	} else {
		for _, filename := range []string{"id_rsa", "id_ed25519"} {
			keyPath := filepath.Join(homeDir, ".ssh", filename)
			key, err := privateKeyFile(keyPath)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(key))
				break
			}
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	config := &ssh.ClientConfig{
		User:              user,
		Auth:              authMethods,
		HostKeyCallback:   kh.HostKeyCallback(),
		HostKeyAlgorithms: kh.HostKeyAlgorithms(addr),
		Timeout:           timeout,
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, nil, fmt.Errorf("dial SSH: %w", err)
	}

	session, err := conn.NewSession()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("new SSH session: %w", err)
	}

	return conn, session, nil
}
