package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func callSSH(a, user, password, host string) error {
	// ssh config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return fmt.Errorf("new knownhosts: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: hostKeyCallback,
	}

	// Connect to SSH server
	conn, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// Create a session
	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	// Create a buffer to capture standard error output
	var stderrBuf = new(bytes.Buffer)
	session.Stderr = stderrBuf

	// Run the command and get the output
	output, err := session.Output(a)
	if err != nil {
		// Print the captured standard error output
		stderrOutput := stderrBuf.String()
		return fmt.Errorf("%w\nStderr Output: %s", err, stderrOutput)
	}

	fmt.Printf("%s\n", output)
	return nil
}

func run(user, password, filePath, host string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		err := callSSH(scanner.Text(), user, password, host)
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
	var userArg, passwordArg, fileArg, hostArg string
	flag.StringVar(&userArg, "user", "", "SSH username")
	flag.StringVar(&passwordArg, "password", "", "SSH password")
	flag.StringVar(&fileArg, "file", "lines.txt", "File containing commands")
	flag.StringVar(&hostArg, "host", "192.168.1.150", "IP address or hostname")
	flag.Parse()

	if userArg == "" {
		currentUser, err := user.Current()
		if err != nil {
			fmt.Println("Error getting current user:", err)
			return
		}
		userArg = currentUser.Username
	}

	if passwordArg == "" {
		fmt.Println("Error: Password is required.")
		return
	}

	if err := run(userArg, passwordArg, fileArg, hostArg); err != nil {
		fmt.Println(err)
	}
}
