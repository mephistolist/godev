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

func main() {
	var userArg, passwordArg, fileArg, hostArg, scriptArg, inventoryArg string
	var portArg, timeoutSeconds, concurrency int
	var promptForPassword bool

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage of %s:
  -f, --file string        File containing commands (default "commands.txt")
  -h, --host string        Single IP address or hostname
  -i, --inventory string   Path to inventory file (must start with "inventory")
  -w, --password           Prompt for SSH password
  -p, --port int           SSH port (default 22)
  -s, --script string      Path to a script or binary to upload and execute
  -t, --timeout int        Timeout in seconds for SSH connection (e.g., 10)
  -u, --user string        SSH username
  -c, --concurrency int    Number of concurrent SSH connections (default 5)
`, os.Args[0])
	}

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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting home directory:", err)
		return
	}

	// Optional: This could be used later in SSH clients for host verification
	// knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")

	if portArg < 1 || portArg > 65335 {
		fmt.Fprintln(os.Stderr, "Error: Port must be between 1 and 65335.")
		return
	}

	if timeoutSeconds < 0 {
		fmt.Fprintln(os.Stderr, "Error: Timeout must be a positive integer.")
		return
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

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
		ok := false
		for _, fn := range []string{"id_rsa", "id_ed25519"} {
			if _, err := os.Stat(filepath.Join(homeDir, ".ssh", fn)); err == nil {
				ok = true
				break
			}
		}
		if !ok {
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
				out, err = client.RunRemoteScript(h.User, h.Password, h.Host, h.Port, timeout, scriptArg)
			} else {
				out, err = client.Run(h.User, h.Password, fileArg, h.Host, h.Port, timeout)
			}

			mu.Lock()
			defer mu.Unlock()

			fmt.Printf("======================================\n")
			if err != nil {
				fmt.Fprintf(os.Stderr, "------ Error with host %s -----\n", h.Host)
				fmt.Fprintf(os.Stderr, "======================================\n%v\n", err)
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
