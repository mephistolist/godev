package client

import (
	"bufio"
	"fmt"
	"os"
	"time"
	"sync"
)

func Run(user, password, filePath, host string, port int, timeout time.Duration) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var output string
	for scanner.Scan() {
		command := scanner.Text()
		if command == "" {
			continue
		}

		var wg sync.WaitGroup
		resultCh := make(chan Result, 1)

		wg.Add(1)
		go callSSH(command, user, password, host, port, timeout, resultCh, &wg)

		wg.Wait()
		close(resultCh)

		result := <-resultCh
		if result.Error != nil {
			output += fmt.Sprintf("Error running command: %v\n", result.Error)
		} else {
			output += fmt.Sprintf("%s\n", result.Output)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scanner error: %w", err)
	}

	return output, nil
}
