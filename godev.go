package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"
	"bytes"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"flag"
)

type Result struct {
	Host   string
	Output string
	Error  error
}

func callSSH(a, user, password, host string, port int, timeout time.Duration, resultCh chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	// ssh config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("get home directory: %w", err)}
		return
	}

	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("new knownhosts: %w", err)}
		return
	}

	var authMethods []ssh.AuthMethod
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	} else {
		// Use SSH keys if password is not provided
		keyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
		key, err := privateKeyFile(keyPath)
		if err != nil {
			resultCh <- Result{Host: host, Error: fmt.Errorf("private key file: %w", err)}
			return
		}
		authMethods = append(authMethods, ssh.PublicKeys(key))
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	// Add a timeout for the SSH connection
	config.Timeout = timeout

	// Connect to SSH server with custom port
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		// Check if the error is due to a timeout
		if isTimeoutError(err) {
			resultCh <- Result{Host: host, Error: fmt.Errorf("Error: Host %s was not reachable within %s", host, timeout)}
		} else {
			resultCh <- Result{Host: host, Error: fmt.Errorf("Error: Host %s encountered an SSH error: %v", host, err)}
		}
		return
	}
	defer conn.Close()

	// Create a session
	session, err := conn.NewSession()
	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("new session: %w", err)}
		return
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
		resultCh <- Result{Host: host, Error: fmt.Errorf("Error running command on host %s: %v\nStderr Output: %s", host, err, stderrOutput)}
		return
	}

	resultCh <- Result{Host: host, Output: string(output)}
}

// Function to check if the error is due to a timeout
func isTimeoutError(err error) bool {
	_, ok := err.(*ssh.ExitMissingError)
	return ok
}

// Function to read private key file
func privateKeyFile(file string) (ssh.Signer, error) {
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	key, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func run(user, password, filePath, host string, port int, timeout time.Duration) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Print host information once
	fmt.Printf("Output from host %s:\n", host)

	var wg sync.WaitGroup
	resultCh := make(chan Result, 1)

	for scanner.Scan() {
		command := scanner.Text()
		wg.Add(1)
		go callSSH(command, user, password, host, port, timeout, resultCh, &wg)
	}

	// Goroutine to close resultCh when all commands are done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Map to store results in the order of commands
	resultsMap := make(map[int]Result)
	index := 0

	// Goroutine to collect results and store them in the map
	go func() {
		for result := range resultCh {
			resultsMap[index] = result
			index++
		}
	}()

	// Wait for all commands to complete
	wg.Wait()

	// Print results in the order of commands
	for i := 0; i < index; i++ {
		result := resultsMap[i]
		if result.Error != nil {
			fmt.Printf("Error running command on host %s: %v\n", host, result.Error)
		} else {
			fmt.Printf("%s\n", result.Output)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func main() {
	var userArg, passwordArg, fileArg, hostArg string
	var portArg int
	var timeoutArg string

	// Define homeDir
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}

	flag.StringVar(&userArg, "user", "", "SSH username")
	flag.StringVar(&passwordArg, "password", "", "SSH password")
	flag.StringVar(&fileArg, "file", "lines.txt", "File containing commands")
	flag.StringVar(&hostArg, "host", "", "IP address or hostname")
	flag.IntVar(&portArg, "port", 22, "SSH port")
	flag.StringVar(&timeoutArg, "timeout", "10s", "Timeout for SSH connection (e.g., 10s)")
	flag.Parse()

	// Check if -host is provided
	if hostArg == "" {
		fmt.Println("Error: -host is required.")
		return
	}

	// Check if the user wants to use keys and password is not provided
	if passwordArg == "" {
		// Check if the private key file exists
		keyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			fmt.Println("Error: Password or private key is required.")
			return
		}
	}

	timeout, err := time.ParseDuration(timeoutArg)
	if err != nil {
		fmt.Println("Error parsing timeout:", err)
		return
	}

	if userArg == "" {
		currentUser, err := user.Current()
		if err != nil {
			fmt.Println("Error getting current user:", err)
			return
		}
		userArg = currentUser.Username
	}

	if err := run(userArg, passwordArg, fileArg, hostArg, portArg, timeout); err != nil {
		fmt.Println(err)
	}
}
