package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
	"godev/client"
)

func main() {
	var userArg, passwordArg, fileArg, hostArg, scriptArg string
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
	flag.StringVar(&timeoutArg, "timeout", "0s", "Timeout for SSH connection (e.g., 10s)")
	flag.IntVar(&portArg, "port", 22, "SSH port")
	flag.BoolVar(&promptForPassword, "password", false, "Prompt for SSH password")
	flag.StringVar(&scriptArg, "script", "", "Path to a script or binary to upload and execute")

	// Allow -h as alias for -host
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

	if userArg == "" {
		currentUser, err := user.Current()
		if err != nil {
			fmt.Println("Error getting current user:", err)
			return
		}
		userArg = currentUser.Username
	}

	type HostInfo struct {
		User string
		Host string
		Port int
	}

	parseInventoryLine := func(line, defaultUser string, defaultPort int) (HostInfo, error) {
		user := defaultUser
		port := defaultPort
		host := line

		if strings.Contains(line, "@") {
			parts := strings.SplitN(line, "@", 2)
			user = parts[0]
			host = parts[1]
		}

		if strings.Contains(host, ":") {
			parts := strings.SplitN(host, ":", 2)
			host = parts[0]
			p, err := strconv.Atoi(parts[1])
			if err != nil {
				return HostInfo{}, fmt.Errorf("invalid port in line: %s", line)
			}
			port = p
		}

		return HostInfo{User: user, Host: host, Port: port}, nil
	}

	var hosts []HostInfo

	if hostArg != "" {
		hosts = append(hosts, HostInfo{User: userArg, Host: hostArg, Port: portArg})
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
			if line == "" {
				continue
			}
			h, err := parseInventoryLine(line, userArg, portArg)
			if err != nil {
				fmt.Printf("Skipping invalid inventory line: %s\n", line)
				continue
			}
			hosts = append(hosts, h)
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading inventory file:", err)
			return
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	sem := make(chan struct{}, 5)

	for _, h := range hosts {
		wg.Add(1)
		go func(h HostInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var output string
			var err error

			if scriptArg != "" {
				output, err = client.RunRemoteScript(h.User, passwordArg, h.Host, h.Port, timeout, scriptArg)
			} else {
				output, err = client.Run(h.User, passwordArg, fileArg, h.Host, h.Port, timeout)
			}

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				fmt.Printf("\n======================================\n")
				fmt.Printf("------ Error with host %s -----\n", h.Host)
				fmt.Printf("======================================\n")
				fmt.Printf("%v", err)
			} else {
				fmt.Printf("\n======================================\n")
				fmt.Printf("----- Output from host %s -----\n", h.Host)
				fmt.Printf("======================================\n\n")
				fmt.Printf("%s", output)
				//fmt.Printf("----------------------------------------\n")
			}
	
			if timeout > 0 {
				time.Sleep(timeout)
			}
		}(h)
	}

	wg.Wait()
	fmt.Printf("\n")
}
