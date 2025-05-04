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
	var timeoutSeconds int
	var promptForPassword bool

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}

	flag.StringVar(&userArg, "user", "", "SSH username")
	flag.StringVar(&fileArg, "file", "commands.txt", "File containing commands")
	flag.StringVar(&hostArg, "host", "", "Single IP address or hostname")
	flag.IntVar(&timeoutSeconds, "timeout", 0, "Timeout in seconds for SSH connection (e.g., 10)")
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

	// Validate the port number
	if portArg < 1 || portArg > 65335 {
		fmt.Println("Error: Port must be between 1 and 65335.")
		return
	}

	if timeoutSeconds < 0 {
		fmt.Println("Error: Timeout must be a positive integer.")
		return
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	// Prompt for password if requested
	if promptForPassword {
		fmt.Print("Password: ")
		p, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Println("Error reading password:", err)
			return
		}
		passwordArg = string(p)
	}

	// If no password from CLI, ensure a key exists
	if passwordArg == "" {
		ok := false
		for _, fn := range []string{"id_rsa", "id_ed25519"} {
			if _, err := os.Stat(filepath.Join(homeDir, ".ssh", fn)); err == nil {
				ok = true
				break
			}
		}
		if !ok {
			fmt.Println("Error: No password provided and no usable private key found.")
			return
		}
	}

	if userArg == "" {
		u, err := user.Current()
		if err != nil {
			fmt.Println("Error getting current user:", err)
			return
		}
		userArg = u.Username
	}

	// parseInventoryLine handles stripping inline comments and blank lines
	parseInventoryLine := func(raw string, defUser string, defPort int) (client.HostInfo, error) {
		line := strings.TrimSpace(raw)
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			return client.HostInfo{}, nil
		}

		user := defUser
		port := defPort
		password := ""
		hostPort := line

		if parts := strings.SplitN(hostPort, "@", 2); len(parts) == 2 {
			user = parts[0]
			hostPort = parts[1]
		}

		if parts := strings.SplitN(hostPort, "::", 2); len(parts) == 2 {
			hostPort = parts[0]
			password = parts[1]
		}

		host := hostPort
		if parts := strings.SplitN(hostPort, ":", 2); len(parts) == 2 {
			host = parts[0]
			if p, err := strconv.Atoi(parts[1]); err == nil {
				port = p
			} else {
				return client.HostInfo{}, fmt.Errorf("invalid port in line %q", raw)
			}
		}

		return client.HostInfo{User: user, Host: host, Port: port, Password: password}, nil
	}

	var hosts []client.HostInfo
	if hostArg != "" {
		hosts = append(hosts, client.HostInfo{
			User:     userArg,
			Host:     hostArg,
			Port:     portArg,
			Password: passwordArg,
		})
	} else {
		f, err := os.Open("inventory")
		if err != nil {
			fmt.Println("Error: inventory file not found and no -host provided.")
			return
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		for s.Scan() {
			h, err := parseInventoryLine(s.Text(), userArg, portArg)
			if err != nil {
				fmt.Printf("Skipping invalid line: %s\n", err)
				continue
			}
			if h.Host != "" {
				hosts = append(hosts, h)
			}
		}
		if err := s.Err(); err != nil {
			fmt.Println("Error reading inventory:", err)
			return
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 5)

	for _, h := range hosts {
		wg.Add(1)
		go func(h client.HostInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var out string
			var err error
			if scriptArg != "" {
				out, err = client.RunRemoteScript(h.User, h.Password, h.Host, h.Port, timeout, scriptArg)
			} else {
				out, err = client.Run(h.User, h.Password, fileArg, h.Host, h.Port, timeout)
			}

			mu.Lock()
			defer mu.Unlock()
			fmt.Printf("======================================\n")
			if err != nil {
				fmt.Printf("------ Error with host %s -----\n", h.Host)
				fmt.Printf("======================================\n%v\n", err)
			} else {
				fmt.Printf("----- Output from host %s -----\n", h.Host)
				fmt.Printf("======================================\n\n%s\n", out)
			}
			if timeout > 0 {
				time.Sleep(timeout)
			}
		}(h)
	}

	wg.Wait()
}
