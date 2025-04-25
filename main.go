package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"godev/client"
)

func main() {
	var userArg, passwordArg, fileArg, hostArg string
	var portArg int
	var timeoutArg string

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

	if hostArg == "" {
		fmt.Println("Error: -host is required.")
		return
	}

	if passwordArg == "" {
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

	if err := client.Run(userArg, passwordArg, fileArg, hostArg, portArg, timeout); err != nil {
		fmt.Println(err)
	}
}
