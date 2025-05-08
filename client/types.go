package client

type HostInfo struct {
	User string
	Host string
	Port int
	Password string
	SudoPassword string
}

type Result struct {
	Host   string
	Output string
	Error  error
}
