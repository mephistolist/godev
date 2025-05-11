package client

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"net"

	"golang.org/x/crypto/ssh"
	skeemakh "github.com/skeema/knownhosts"
)

// Updated callSSH with allowUnknownHosts
func callSSH(command, user, password, host string, port int, timeout time.Duration, allowUnknownHosts bool, resultCh chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("get home directory: %w", err)}
		return
	}

	khPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	kh, err := skeemakh.NewDB(khPath)
	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("load known_hosts DB: %w", err)}
		return
	}

	var authMethods []ssh.AuthMethod
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	} else {
		keyTried := false
		for _, filename := range []string{"id_rsa", "id_ed25519"} {
			keyPath := filepath.Join(homeDir, ".ssh", filename)
			key, err := privateKeyFile(keyPath)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(key))
				keyTried = true
				break
			}
		}
		if !keyTried {
			resultCh <- Result{Host: host, Error: fmt.Errorf("no usable private key found in ~/.ssh")}
			return
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	config := &ssh.ClientConfig{
		User:              user,
		Auth:              authMethods,
		HostKeyCallback:   getHostKeyCallback(allowUnknownHosts),
		HostKeyAlgorithms: kh.HostKeyAlgorithms(addr),
		//Timeout:           timeout,
	}

	//conn, err := ssh.Dial("tcp", addr, config)
	netDialer := net.Dialer{Timeout: timeout}
	connRaw, err := netDialer.Dial("tcp", addr)
	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("dial TCP: %w", err)}
		return
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(connRaw, addr, config)
	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("new client connection: %w", err)}
		connRaw.Close()
		return
	}
	conn := ssh.NewClient(clientConn, chans, reqs)


	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("dial SSH: %w", err)}
		return
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		resultCh <- Result{Host: host, Error: fmt.Errorf("new session: %w", err)}
		return
	}
	defer session.Close()

	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	output, err := session.Output(command)
	if err != nil {
		stderrOutput := stderrBuf.String()
		resultCh <- Result{Host: host, Error: fmt.Errorf("command error: %v; stderr: %s", err, stderrOutput)}
		return
	}

	combinedOutput := string(output)
	if stderrBuf.Len() > 0 {
		combinedOutput += "\n" + stderrBuf.String()
	}

	resultCh <- Result{Host: host, Output: combinedOutput}
}
