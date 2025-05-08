package main

import (
	"bufio"
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
	"github.com/spf13/pflag"
)

func parseInventoryLine(raw string, defUser string, defPort int) (client.HostInfo, error) {
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
	sudoPassword := ""
	hostPort := line

	if parts := strings.SplitN(hostPort, ":::", 2); len(parts) == 2 {
		hostPort, sudoPassword = parts[0], parts[1]
	}
	if parts := strings.SplitN(hostPort, "::", 2); len(parts) == 2 {
		hostPort, password = parts[0], parts[1]
	}
	if parts := strings.SplitN(hostPort, "@", 2); len(parts) == 2 {
		user, hostPort = parts[0], parts[1]
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

	return client.HostInfo{
		User:         user,
		Host:         host,
		Port:         port,
		Password:     password,
		SudoPassword: sudoPassword,
	}, nil
}

func main() {
	var userArg, passwordArg, fileArg, hostArg, scriptArg, inventoryArg string
	var portArg, timeoutSeconds, concurrency int
	var promptForPassword bool

	pflag.StringVarP(&userArg, "user", "u", "", "SSH username")
	pflag.StringVarP(&fileArg, "file", "f", "commands.txt", "File containing commands")
	pflag.StringVarP(&hostArg, "host", "h", "", "Single IP address or hostname")
	pflag.StringVarP(&inventoryArg, "inventory", "i", "inventory", "Path to inventory file")
	pflag.IntVarP(&timeoutSeconds, "timeout", "t", 0, "Timeout in seconds for SSH connection")
	pflag.IntVarP(&portArg, "port", "p", 22, "SSH port")
	pflag.BoolVarP(&promptForPassword, "password", "w", false, "Prompt for SSH password")
	pflag.StringVarP(&scriptArg, "script", "s", "", "Path to a script or binary to upload and execute")
	pflag.IntVarP(&concurrency, "concurrency", "c", 5, "Max concurrent SSH connections")
	pflag.Parse()

	if !strings.HasPrefix(filepath.Base(inventoryArg), "inventory") {
		fmt.Fprintf(os.Stderr, "Error: Inventory file must start with \"inventory\" (got: %q)\n", inventoryArg)
		os.Exit(1)
	}

	if portArg < 1 || portArg > 65335 {
		fmt.Fprintln(os.Stderr, "Error: Port must be between 1 and 65335.")
		return
	}
	if timeoutSeconds < 0 {
		fmt.Fprintln(os.Stderr, "Error: Timeout must be a positive integer.")
		return
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting home directory:", err)
		return
	}

	if promptForPassword {
		fmt.Print("Password: ")
		p, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading password:", err)
			return
		}
		passwordArg = string(p)
	}

	if passwordArg == "" {
		found := false
		for _, fn := range []string{"id_rsa", "id_ed25519"} {
			if _, err := os.Stat(filepath.Join(homeDir, ".ssh", fn)); err == nil {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintln(os.Stderr, "Error: No password provided and no usable private key found.")
			return
		}
	}

	if userArg == "" {
		u, err := user.Current()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error getting current user:", err)
			return
		}
		userArg = u.Username
	}

	var hosts []client.HostInfo
	if hostArg != "" {
		hosts = append(hosts, client.HostInfo{
			User:         userArg,
			Host:         hostArg,
			Port:         portArg,
			Password:     passwordArg,
			SudoPassword: "",
		})
	} else {
		f, err := os.Open(inventoryArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: inventory file %q not found and no -host provided.\n", inventoryArg)
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			h, err := parseInventoryLine(scanner.Text(), userArg, portArg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Skipping invalid line: %s\n", err)
				continue
			}
			if h.Host != "" {
				hosts = append(hosts, h)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Error reading inventory:", err)
			return
		}
	}

	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No valid hosts found in inventory file or supplied with the -host option.")
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)

	for _, h := range hosts {
		wg.Add(1)
		go func(h client.HostInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var out string
			var err error
			if scriptArg != "" {
				out, err = client.RunRemoteScriptWithSudo(h.User, h.Password, h.SudoPassword, h.Host, h.Port, timeout, scriptArg)
			} else {
				out, err = client.Run(h.User, h.Password, fileArg, h.Host, h.Port, timeout)
			}

			mu.Lock()
			defer mu.Unlock()

			fmt.Println("======================================")
			fmt.Printf("----- Output from host %s -----\n", h.Host)
			fmt.Println("======================================\n")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error:\n%v\n\n", err)
			} else {
				fmt.Println(out)
			}
		}(h)
	}

	wg.Wait()
}
