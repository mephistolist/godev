package client

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func getHostKeyCallback(allowUnknown bool) ssh.HostKeyCallback {
	if allowUnknown {
		// Print a loud warning on stderr
		fmt.Fprintln(os.Stderr, "[WARN] SSH: Skipping host key verification (INSECURE MODE)")
		return ssh.InsecureIgnoreHostKey()
	}

	// Default behavior (check known_hosts)
	kh, err := knownhosts.New(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error loading known_hosts:", err)
		return ssh.InsecureIgnoreHostKey() // fallback if needed
	}
	return kh
}

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

func isTimeoutError(err error) bool {
	_, ok := err.(*ssh.ExitMissingError)
	return ok
}
