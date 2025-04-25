package client

import (
  "bytes"
  "fmt"
  "os"
  "path/filepath"
  "sync"
  "time"

  "golang.org/x/crypto/ssh"
  skeemakh "github.com/skeema/knownhosts"
)

func callSSH(command, user, password, host string, port int, timeout time.Duration, resultCh chan<- Result, wg *sync.WaitGroup) {
  defer wg.Done()

  homeDir, err := os.UserHomeDir()
  if err != nil {
    resultCh <- Result{Host: host, Error: fmt.Errorf("get home directory: %w", err)}
    return
  }

  // 1) Load skeema known_hosts DB
  khPath := filepath.Join(homeDir, ".ssh", "known_hosts")
  kh, err := skeemakh.NewDB(khPath)
  if err != nil {
    resultCh <- Result{Host: host, Error: fmt.Errorf("load known_hosts DB: %w", err)}
    return
  }

  // 2) Build ssh.ClientConfig with callback & algorithms
  var authMethods []ssh.AuthMethod
  if password != "" {
    authMethods = append(authMethods, ssh.Password(password))
  } else {
    keyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
    key, err := privateKeyFile(keyPath)
    if err != nil {
      resultCh <- Result{Host: host, Error: fmt.Errorf("private key file: %w", err)}
      return
    }
    authMethods = append(authMethods, ssh.PublicKeys(key))
  }

  addr := fmt.Sprintf("%s:%d", host, port)
  config := &ssh.ClientConfig{
    User:              user,
    Auth:              authMethods,
    HostKeyCallback:   kh.HostKeyCallback(),
    HostKeyAlgorithms: kh.HostKeyAlgorithms(addr),
    Timeout:           timeout,
  }

  // 3) Dial and run
  conn, err := ssh.Dial("tcp", addr, config)
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
    resultCh <- Result{Host: host, Error: fmt.Errorf("command error: %v; stderr: %s", err, stderrBuf.String())}
    return
  }

  resultCh <- Result{Host: host, Output: string(output)}
}
