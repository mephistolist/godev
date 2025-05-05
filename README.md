# GoDev
A simple cross-platform devops project in Golang that's built for speed and customization.

$ godev --help
Usage of godev:
  -f, --file string       File containing commands (default "commands.txt")
  -h, --host string       Single IP address or hostname
  -i, --inventory string  Path to inventory file (must start with "inventory")
  -w, --password          Prompt for SSH password
  -p, --port int          SSH port (default 22)
  -s, --script string     Path to a script or binary to upload and execute
  -t, --timeout int       Timeout in seconds for SSH connection (e.g., 10)
  -u, --user string       SSH username

# Todo
Keep checking semgrep.<br>
Refactor for performance.<br>
Ponder making WinSync part of the rest of the code.<br>
Create installer script and polish README.<br>
