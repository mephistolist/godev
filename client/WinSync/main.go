package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/user"
    "time"
    "io"
    "path/filepath"
    "net"
    "strings"

    "golang.org/x/crypto/ssh"
    "golang.org/x/crypto/ssh/agent"
    "github.com/pkg/sftp"
)

func main() {
    me, err := user.Current()
    if err != nil {
        log.Fatalf("cannot determine current user: %v", err)
    }

    userFlag := flag.String("user", me.Username, "Username for remote connection (default: current user)")
    passwordFlag := flag.String("password", "", "Password for remote login (optional)")
    host := flag.String("host", "", "Remote host (required)")
    port := flag.Int("port", 22, "SSH port (default 22)")
    timeoutSeconds := flag.Int("timeout", 30, "Connection timeout in seconds")
    script := flag.String("script", "", "Local script to upload and run (required)")

    flag.Parse()

    if *host == "" || *script == "" {
        flag.Usage()
        log.Fatal("host and script options are required")
    }

    output, err := RunWindowsRemoteScript(*userFlag, *passwordFlag, *host, *port, time.Duration(*timeoutSeconds)*time.Second, *script)
    if err != nil {
        log.Fatalf("remote script failed: %v", err)
    }

    fmt.Print(output)
}

func RunWindowsRemoteScript(user, password, host string, port int, timeout time.Duration, scriptPath string) (string, error) {
    scriptName := filepath.Base(scriptPath)
    remotePath := "C:\\tmp\\" + scriptName

    mkTmpCmd := `powershell -Command "if (!(Test-Path C:\\tmp)) { New-Item -ItemType Directory -Path C:\\tmp }"`
    if _, err := RunCommand(user, password, host, port, timeout, mkTmpCmd); err != nil {
        return "", fmt.Errorf("failed to create C:\\tmp: %v", err)
    }

    if err := SFTPUpload(user, password, host, port, timeout, scriptPath, remotePath); err != nil {
        return "", fmt.Errorf("sftp upload failed: %v", err)
    }

    execCmd := fmt.Sprintf(`cmd /C "C:\\tmp\\%s"`, scriptName)
    return RunCommand(user, password, host, port, timeout, execCmd)
}

func SFTPUpload(user, password, host string, port int, timeout time.Duration, localPath, remotePath string) error {
    client, err := connectSSH(user, password, host, port, timeout)
    if err != nil {
        return err
    }
    defer client.Close()

    sftpClient, err := sftp.NewClient(client)
    if err != nil {
        return fmt.Errorf("failed to start sftp subsystem: %v", err)
    }
    defer sftpClient.Close()

    dstFile, err := sftpClient.Create(remotePath)
    if err != nil {
        return fmt.Errorf("cannot create remote file: %v", err)
    }
    defer dstFile.Close()

    srcFile, err := os.Open(localPath)
    if err != nil {
        return fmt.Errorf("cannot open local file: %v", err)
    }
    defer srcFile.Close()

    if _, err := io.Copy(dstFile, srcFile); err != nil {
        return fmt.Errorf("copy failed: %v", err)
    }
    return nil
}

func RunCommand(user, password, host string, port int, timeout time.Duration, cmd string) (string, error) {
    client, err := connectSSH(user, password, host, port, timeout)
    if err != nil {
        return "", err
    }
    defer client.Close()

    session, err := client.NewSession()
    if err != nil {
        return "", fmt.Errorf("failed to create session: %v", err)
    }
    defer session.Close()

    out, err := session.CombinedOutput(cmd)
    return string(out), err
}

func connectSSH(user, password, host string, port int, timeout time.Duration) (*ssh.Client, error) {
    var methods []ssh.AuthMethod

    // Try SSH Agent first
    if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
        if conn, err := net.Dial("unix", sock); err == nil {
            agentClient := agent.NewClient(conn)
            methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
        }
    }

    // Password authentication
    if strings.TrimSpace(password) != "" {
        methods = append(methods, ssh.Password(password))
    }

    if len(methods) == 0 {
        return nil, fmt.Errorf("no authentication methods available (no password or SSH agent)")
    }

    config := &ssh.ClientConfig{
        User:            user,
        Auth:            methods,
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         timeout,
    }

    addr := fmt.Sprintf("%s:%d", host, port)
    return ssh.Dial("tcp", addr, config)
}
