package main

import (
	"bufio"
	"fmt"
	"bytes"
	"os"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func callSSH(a string) error {
	// ssh config
	hostKeyCallback, err := knownhosts.New("/home/user/.ssh/known_hosts")
	if err != nil {
		return fmt.Errorf("new knownhosts: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            "user",
		Auth:            []ssh.AuthMethod{ssh.Password("password")},
		HostKeyCallback: hostKeyCallback,
	}

	// Are you sure you want to open a new connection for every call?
	// connect to ssh server
	conn, err := ssh.Dial("tcp", "10.20.50.6:22", config)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// create a session
	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	// Create a buffer to capture the standard error output
	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	// run the command and get the output
	output, err := session.Output(a)
	if err != nil {
		// Print the captured standard error output
		stderrOutput := stderrBuf.String()
		return fmt.Errorf("%w\nStderr Output: %s", err, stderrOutput)
	}

	fmt.Printf("%s\n", output)
	return nil
}

func run() error {
	file, err := os.Open("lines.txt")
	if err != nil {
		return fmt.Errorf("open lines file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		err := callSSH(scanner.Text())
		if err != nil {
			return fmt.Errorf("call ssh: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
	}
}
