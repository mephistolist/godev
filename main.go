package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
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
	flag.StringVar(&hostArg, "host", "", "IP address or hostname")
	flag.StringVar(&timeoutArg, "timeout", "10s", "Timeout for SSH connection (e.g., 10s)")
	flag.IntVar(&portArg, "port", 22, "SSH port")
	flag.BoolVar(&promptForPassword, "password", false, "Prompt for SSH password (optional)")

	// Allow -h as an alias for -host
	for i, arg := range os.Args {
		if arg == "-h" && i+1 < len(os.Args) {
			os.Args[i] = "-host"
		}
	}

	flag.Parse()

	// Required flag
	if hostArg == "" {
		fmt.Println("Error: -host is required.")
		flag.Usage()
		return
	}

	// Prompt for password if the flag was passed
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

	// If no password, ensure key exists
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

	// Run the client logic
	if err := client.Run(userArg, passwordArg, fileArg, hostArg, portArg, timeout); err != nil {
		fmt.Println(err)
	}
}
