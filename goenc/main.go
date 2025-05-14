package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
	"github.com/spf13/pflag"
)

func encrypt(plaintext, password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}

	key := pbkdf2.Key([]byte(password), salt, 100_000, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	ciphertext := make([]byte, len(plaintext))
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext, []byte(plaintext))

	result := append(salt, iv...)
	result = append(result, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

func decrypt(encryptedBase64, password string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", err
	}
	if len(data) < 32 {
		return "", errors.New("encrypted data too short")
	}

	salt := data[:16]
	iv := data[16:32]
	ciphertext := data[32:]

	key := pbkdf2.Key([]byte(password), salt, 100_000, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

func promptPassword(confirm bool) (string, error) {
	fmt.Print("Enter password: ")
	pw1, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}

	if confirm {
		fmt.Print("Confirm password: ")
		pw2, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", err
		}
		if string(pw1) != string(pw2) {
			return "", errors.New("passwords do not match")
		}
	}

	return string(pw1), nil
}

func main() {
	var encryptPath, decryptPath string

	pflag.StringVarP(&encryptPath, "encrypt", "e", "", "Encrypt the specified file")
	pflag.StringVarP(&decryptPath, "decrypt", "d", "", "Decrypt the specified file")
	pflag.Parse()

	if (encryptPath == "" && decryptPath == "") || (encryptPath != "" && decryptPath != "") {
		fmt.Println("Usage:")
		fmt.Println("  -e, --encrypt <path>   Encrypt a file")
		fmt.Println("  -d, --decrypt <path>   Decrypt a file")
		os.Exit(1)
	}

	if encryptPath != "" {
		pw, err := promptPassword(true)
		if err != nil {
			log.Fatalf("Password error: %v", err)
		}
		data, err := os.ReadFile(encryptPath)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}
		encrypted, err := encrypt(string(data), pw)
		if err != nil {
			log.Fatalf("Encryption failed: %v", err)
		}
		if err := os.WriteFile(encryptPath, []byte(encrypted), 0o600); err != nil {
			log.Fatalf("Failed to write encrypted file: %v", err)
		}
		fmt.Println("Encryption successful.")
		return
	}

	if decryptPath != "" {
		data, err := os.ReadFile(decryptPath)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}
		for i := 0; i < 3; i++ {
			pw, err := promptPassword(false)
			if err != nil {
				log.Fatalf("Password error: %v", err)
			}
			plain, err := decrypt(string(data), pw)
			if err == nil {
				if err := os.WriteFile(decryptPath, []byte(plain), 0o600); err != nil {
					log.Fatalf("Failed to write decrypted file: %v", err)
				}
				fmt.Println("Decryption successful.")
				return
			}
			fmt.Println("Incorrect password. Try again.")
		}
		log.Fatal("Maximum password attempts reached. Decryption failed.")
	}
}
