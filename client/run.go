package client

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

func Run(user, password, filePath, host string, port int, timeout time.Duration) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	fmt.Printf("Output from host %s:\n", host)

	var wg sync.WaitGroup
	resultCh := make(chan Result, 1)

	for scanner.Scan() {
		command := scanner.Text()
		wg.Add(1)
		go callSSH(command, user, password, host, port, timeout, resultCh, &wg)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	resultsMap := make(map[int]Result)
	index := 0

	go func() {
		for result := range resultCh {
			resultsMap[index] = result
			index++
		}
	}()

	wg.Wait()

	for i := 0; i < index; i++ {
		result := resultsMap[i]
		if result.Error != nil {
			fmt.Printf("Error running command on host %s: %v\n", host, result.Error)
		} else {
			fmt.Printf("%s\n", result.Output)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}
