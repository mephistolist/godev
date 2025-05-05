# GoDev
A simple cross-platform devops project in Golang that's built for speed and customization.

$ godev --help<br>
Usage of godev:<br>
  -f, --file string       File containing commands (default "commands.txt")<br>
  -h, --host string       Single IP address or hostname<br>
  -i, --inventory string  Path to inventory file (must start with "inventory")<br>
  -w, --password          Prompt for SSH password<br>
  -p, --port int          SSH port (default 22)<br>
  -s, --script string     Path to a script or binary to upload and execute<br>
  -t, --timeout int       Timeout in seconds for SSH connection (e.g., 10)<br>
  -u, --user string       SSH username<br>

# Todo
Keep checking semgrep.<br>
Refactor for performance.<br>
Ponder making WinSync part of the rest of the code.<br>
Create installer script and polish README.<br>
