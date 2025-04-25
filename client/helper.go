package client

import (
	"os"
	"golang.org/x/crypto/ssh"
)

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
