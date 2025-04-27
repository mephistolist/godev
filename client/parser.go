package client

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseInventoryLine(line, defaultUser string, defaultPort int) (HostInfo, error) {
	user := defaultUser
	port := defaultPort
	host := line

	if strings.Contains(line, "@") {
		parts := strings.SplitN(line, "@", 2)
		user = parts[0]
		host = parts[1]
	}

	if strings.Contains(host, ":") {
		parts := strings.SplitN(host, ":", 2)
		host = parts[0]
		p, err := strconv.Atoi(parts[1])
		if err != nil {
			return HostInfo{}, fmt.Errorf("invalid port in line: %s", line)
		}
		port = p
	}

	return HostInfo{
		User: user,
		Host: host,
		Port: port,
	}, nil
}
