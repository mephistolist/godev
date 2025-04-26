package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
	"godev/client"
)

func main() {
	var userArg, passwordArg, fileArg, hostArg string
	var portArg int
	var timeoutArg string
	var promptForPassword bool

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}

	flag.StringVar(&userArg, "user", "", "SSH username")
	flag.StringVar(&fileArg, "file", "lines.txt", "File containing commands")
	flag.StringVar(&hostArg, "host", "", "Single IP address or hostname")
	flag.StringVar(&timeoutArg, "timeout", "10s", "Timeout for SSH connection (e.g., 10s)")
	flag.IntVar(&portArg, "port", 22, "SSH port")
	flag.BoolVar(&promptForPassword, "password", false, "Prompt for SSH password")

	// Allow -h as an alias for -host
	for i, arg := range os.Args {
		if arg == "-h" && i+1 < len(os.Args) {
			os.Args[i] = "-host"
		}
	}

	flag.Parse()

	// Prompt for password if requested
	if promptForPassword {
		fmt.Print("Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Println("Error reading password:", err)
			return
		}
		passwordArg = string(bytePassword)
	}

	// If no password, check for usable private key
	if passwordArg == "" {
		foundKey := false
		for _, filename := range []string{"id_rsa", "id_ed25519"} {
			keyPath := filepath.Join(homeDir, ".ssh", filename)
			if _, err := os.Stat(keyPath); err == nil {
				foundKey = true
				break
			}
		}
		if !foundKey {
			fmt.Println("Error: No password provided and no usable private key found.")
			return
		}
	}

	timeout, err := time.ParseDuration(timeoutArg)
	if err != nil {
		fmt.Println("Error parsing timeout:", err)
		return
	}

	// Determine user if not provided
	if userArg == "" {
		currentUser, err := user.Current()
		if err != nil {
			fmt.Println("Error getting current user:", err)
			return
		}
		userArg = currentUser.Username
	}

	// Get list of hosts
	var hosts []string
	if hostArg != "" {
		hosts = append(hosts, hostArg)
	} else {
		f, err := os.Open("inventory")
		if err != nil {
			fmt.Println("Error: No -host provided and inventory file not found.")
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				hosts = append(hosts, line)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading inventory file:", err)
			return
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Optional: limit concurrency
	sem := make(chan struct{}, 5) // max 5 concurrent SSH connections

	for _, h := range hosts {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			output, err := client.Run(userArg, passwordArg, fileArg, host, portArg, timeout)

			mu.Lock()
			defer mu.Unlock()

			fmt.Printf("\n=== Connecting to host: %s ===\n", host)
			if err != nil {
				fmt.Printf("Error with host %s: %v\n", host, err)
			} else {
				fmt.Printf("\n%s", output)
			}
		}(h)
	}

	wg.Wait()
}
