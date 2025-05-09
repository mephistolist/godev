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

func splitUnescaped(s string, sep string) []string {
	var parts []string
	var curr strings.Builder
	escaped := false
	i := 0
	for i < len(s) {
		if s[i] == '\\' && !escaped {
			escaped = true
			i++
			continue
		}
		if !escaped && strings.HasPrefix(s[i:], sep) {
			parts = append(parts, curr.String())
			curr.Reset()
			i += len(sep)
			continue
		}
		if escaped {
			curr.WriteByte('\\')
			escaped = false
		}
		curr.WriteByte(s[i])
		i++
	}
	parts = append(parts, curr.String())
	return parts
}

func unescapeField(s string) string {
	replacer := strings.NewReplacer(
		`\\`, `\`,
		`\@`, `@`,
		`\:`, `:`,
		`\#`, `#`,
	)
	return replacer.Replace(s)
}

func parseInventoryLine(raw string, defUser string, defPort int) (client.HostInfo, error) {
	line := strings.TrimSpace(raw)

	commentIndex := -1
	inEscape := false
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' {
			inEscape = !inEscape
		} else {
			if line[i] == '#' && !inEscape {
				commentIndex = i
				break
			}
			inEscape = false
		}
	}
	if commentIndex >= 0 {
		line = strings.TrimSpace(line[:commentIndex])
	}
	if line == "" {
		return client.HostInfo{}, nil
	}

	info := client.HostInfo{
		User: defUser,
		Port: defPort,
	}

	parts := splitUnescaped(line, ":::")
	if len(parts) == 2 {
		line = parts[0]
		info.SudoPassword = unescapeField(parts[1])
	}

	parts = splitUnescaped(line, "::")
	if len(parts) == 2 {
		line = parts[0]
		info.Password = unescapeField(parts[1])
	}

	parts = splitUnescaped(line, "@")
	if len(parts) == 2 {
		info.User = unescapeField(parts[0])
		line = parts[1]
	}

	parts = splitUnescaped(line, ":")
	if len(parts) == 2 {
		info.Host = unescapeField(parts[0])
		portStr := unescapeField(parts[1])
		if p, err := strconv.Atoi(portStr); err == nil {
			info.Port = p
		} else {
			return client.HostInfo{}, fmt.Errorf("invalid port in line %q", raw)
		}
	} else {
		info.Host = unescapeField(line)
	}

	return info, nil
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

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		pflag.PrintDefaults()
	}

	pflag.Parse()

	fileUsed := pflag.Lookup("file").Changed
	scriptUsed := pflag.Lookup("script").Changed

	if !fileUsed && !scriptUsed {
		fmt.Fprintln(os.Stderr, "Error: Either --file or --script must be provided.")
		pflag.Usage()
		os.Exit(1)
	}

	if !strings.HasPrefix(filepath.Base(inventoryArg), "inventory") {
		fmt.Fprintf(os.Stderr, "Error: Inventory file must start with \"inventory\" (got: %q)\n", inventoryArg)
		os.Exit(1)
	}

	if portArg < 1 || portArg > 65335 {
		fmt.Fprintln(os.Stderr, "Error: Port must be between 1 and 65335.")
		os.Exit(1)
	}
	if timeoutSeconds < 0 {
		fmt.Fprintln(os.Stderr, "Error: Timeout must be a positive integer.")
		os.Exit(1)
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting home directory:", err)
		os.Exit(1)
	}

	if promptForPassword {
		fmt.Print("Password: ")
		p, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading password:", err)
			os.Exit(1)
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
			os.Exit(1)
		}
	}

	if userArg == "" {
		u, err := user.Current()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error getting current user:", err)
			os.Exit(1)
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
			os.Exit(1)
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
			os.Exit(1)
		}
	}

	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No valid hosts found in inventory file or supplied with the -host option.")
		os.Exit(1)
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
			if scriptUsed {
				out, err = client.RunRemoteScriptWithSudo(h.User, h.Password, h.SudoPassword, h.Host, h.Port, timeout, scriptArg)
			} else {
				out, err = client.Run(h.User, h.Password, fileArg, h.Host, h.Port, timeout)
			}

			mu.Lock()
			defer mu.Unlock()
			fmt.Printf("======================================\n")
			if err != nil {
				fmt.Printf("------ Error with host %s -----\n", h.Host)
				fmt.Printf("======================================\n\n%v\n", err)
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
