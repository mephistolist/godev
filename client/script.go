package client

import (
    "bytes"
    "fmt"
    "io"
    "net"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/pkg/sftp"
    "golang.org/x/crypto/ssh"
    "golang.org/x/crypto/ssh/agent"
)

// dialSSH establishes an SSH connection (client only).
func dialSSH(user, password, host string, port int, timeout time.Duration) (*ssh.Client, error) {
    var methods []ssh.AuthMethod

    if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
        if c, err := net.Dial("unix", sock); err == nil {
            methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(c).Signers))
        }
    }
    if strings.TrimSpace(password) != "" {
        methods = append(methods, ssh.Password(password))
    }
    if len(methods) == 0 {
        return nil, fmt.Errorf("no authentication methods available")
    }

    cfg := &ssh.ClientConfig{
        User:            user,
        Auth:            methods,
        HostKeyCallback: ssh.InsecureIgnoreHostKey(), 
    }

    addr := fmt.Sprintf("%s:%d", host, port)
    netDialer := net.Dialer{Timeout: timeout}
    conn, err := netDialer.Dial("tcp", addr)
    if err != nil {
        return nil, fmt.Errorf("tcp dial error: %w", err)
    }

    c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("ssh client conn: %w", err)
    }

    return ssh.NewClient(c, chans, reqs), nil
}

// runSSH runs a command over SSH and returns its stdout.
func runSSH(user, password, host string, port int, timeout time.Duration, cmd string) (string, error) {
    conn, err := dialSSH(user, password, host, port, timeout)
    if err != nil {
        return "", err
    }
    defer conn.Close()

    session, err := conn.NewSession()
    if err != nil {
        return "", fmt.Errorf("new session: %v", err)
    }
    defer session.Close()

    var out, errbuf bytes.Buffer
    session.Stdout = &out
    session.Stderr = &errbuf

    if err := session.Run(cmd); err != nil {
        return "", fmt.Errorf("ssh error: %v, stderr: %s", err, errbuf.String())
    }
    return out.String(), nil
}

// tryUpload first attempts rsync; on failure (e.g. Windows), falls back to SFTP.
func tryUpload(user, password, host string, port int, localPath, remotePath string, timeout time.Duration) (bool, error) {
    rsync := exec.Command("rsync", "-e", fmt.Sprintf("ssh -p %d", port), localPath, fmt.Sprintf("%s@%s:%s", user, host, remotePath))
    if err := rsync.Run(); err == nil {
        return false, nil // rsync succeeded → likely Unix
    }
    // fallback to SFTP
    err := sftpUpload(user, password, host, port, timeout, localPath, remotePath)
    return true, err // SFTP used → likely Windows
}

// sftpUpload pushes a file via the SFTP subsystem.
func sftpUpload(user, password, host string, port int, timeout time.Duration, localPath, remotePath string) error {
    conn, err := dialSSH(user, password, host, port, timeout)
    if err != nil {
        return err
    }
    defer conn.Close()

    client, err := sftp.NewClient(conn)
    if err != nil {
        return fmt.Errorf("start sftp: %v", err)
    }
    defer client.Close()

    dst, err := client.Create(remotePath)
    if err != nil {
        return fmt.Errorf("create remote file: %v", err)
    }
    defer dst.Close()

    src, err := os.Open(localPath)
    if err != nil {
        return fmt.Errorf("open local file: %v", err)
    }
    defer src.Close()

    if _, err := io.Copy(dst, src); err != nil {
        return fmt.Errorf("copy file: %v", err)
    }
    return nil
}

// RunRemoteScript uploads and runs a Unix-style script (.sh, no extension, etc).
func RunRemoteScript(user, password, host string, port int, timeout time.Duration, scriptPath string) (string, error) {
    scriptName := filepath.Base(scriptPath)
    remote := "/tmp/" + scriptName

    isWindows, err := tryUpload(user, password, host, port, scriptPath, remote, timeout)
    if err != nil {
        return "", err
    }

    // Only chmod if it's not a Windows host
    if !isWindows {
        if _, err := runSSH(user, password, host, port, timeout, fmt.Sprintf("chmod +x %s", remote)); err != nil {
            return "", fmt.Errorf("chmod failed: %v", err)
        }
    }

    return runSSH(user, password, host, port, timeout, remote)
}

// runSSHWithPTYAndStdin requests a PTY, then runs cmd feeding stdin, and hides sudo prompt.
func runSSHWithPTYAndStdin(
    user, password, host string,
    port int, timeout time.Duration,
    cmd, stdin string,
) (string, error) {
    conn, err := dialSSH(user, password, host, port, timeout)
    if err != nil {
        return "", err
    }
    defer conn.Close()

    session, err := conn.NewSession()
    if err != nil {
        return "", fmt.Errorf("new session: %v", err)
    }
    defer session.Close()

    // request PTY so sudo can run
    if err := session.RequestPty("xterm", 80, 40, ssh.TerminalModes{}); err != nil {
        return "", fmt.Errorf("request pty failed: %v", err)
    }

    var stdout bytes.Buffer
    var stderr bytes.Buffer

    session.Stdout = &stdout
    session.Stderr = &stderr

    // use a pipe to feed password silently
    stdinPipe, err := session.StdinPipe()
    if err != nil {
        return "", fmt.Errorf("stdin pipe: %v", err)
    }

    // run the command
    if err := session.Start(cmd); err != nil {
        return "", fmt.Errorf("ssh start error: %v", err)
    }

    // write sudo password (ends with newline)
    io.WriteString(stdinPipe, stdin)

    // wait for command to finish
    if err := session.Wait(); err != nil {
        // if command failed, you might want stderr in the message
        return "", fmt.Errorf("ssh error: %v, stderr: %s", err, stderr.String())
    }

    // suppress sudo password prompt line in stderr
    cleaned := strings.ReplaceAll(stdout.String(), stdin, "")
    return strings.TrimSpace(cleaned), nil
}

// runSSHWithPTYAndStdin requests a PTY, then runs cmd feeding stdin.
func RunRemoteScriptWithSudo(
    user, sshPass, sudoPass, host string,
    port int,
    timeout time.Duration,
    scriptPath string,
) (string, error) {
    scriptName := filepath.Base(scriptPath)
    remote := "/tmp/" + scriptName

    // upload (rsync→SFTP)
    isWindows, err := tryUpload(user, sshPass, host, port, scriptPath, remote, timeout)
    if err != nil {
        return "", err
    }

    // chmod only if Unix-style
    if !isWindows {
        if _, err := runSSH(user, sshPass, host, port, timeout,
            fmt.Sprintf("chmod +x %s", remote),
        ); err != nil {
            return "", fmt.Errorf("chmod failed: %v", err)
        }
    }

    // if no sudo password, run directly
    if strings.TrimSpace(sudoPass) == "" {
        return runSSH(user, sshPass, host, port, timeout, remote)
    }

    // run with sudo on Unix
    return runSSHWithStdin(
        user, sshPass, host, port, timeout,
        fmt.Sprintf("sudo -S %s", remote),
        sudoPass+"\n",
    )
}

// RunWindowsRemoteScript uploads and runs a Windows batch via SFTP + cmd.
func RunWindowsRemoteScript(user, password, host string, port int, timeout time.Duration, scriptPath string) (string, error) {
    scriptName := filepath.Base(scriptPath)
    remote := "C:\\tmp\\" + scriptName

    // ensure C:\tmp exists
    if _, err := runSSH(user, password, host, port, timeout,
        `powershell -Command "if (!(Test-Path C:\\tmp)) { New-Item -ItemType Directory -Path C:\\tmp }"`); err != nil {
        return "", fmt.Errorf("mk tmp dir: %v", err)
    }

    _, err := tryUpload(user, password, host, port, scriptPath, remote, timeout)
    if err != nil {
        return "", fmt.Errorf("upload script: %v", err)
    }

    return runSSH(user, password, host, port, timeout, fmt.Sprintf(`cmd /C "%s"`, remote))
}

// runSSHWithStdin runs a command feeding stdin.
func runSSHWithStdin(user, password, host string, port int, timeout time.Duration, cmd, stdin string) (string, error) {
    conn, err := dialSSH(user, password, host, port, timeout)
    if err != nil {
        return "", err
    }
    defer conn.Close()

    session, err := conn.NewSession()
    if err != nil {
        return "", fmt.Errorf("new session: %v", err)
    }
    defer session.Close()

    var out, stderr bytes.Buffer
    session.Stdout = &out
    session.Stderr = &stderr
    session.Stdin = strings.NewReader(stdin)

    if err := session.Run(cmd); err != nil {
        return "", fmt.Errorf("ssh error: %v, stderr: %s", err, stderr.String())
    }
    return out.String(), nil
}
