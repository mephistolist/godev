# GoDev
A simple cross-platform devops project in Golang that's built for speed and customization. 

Since this is written with golang, you can use this program for Windows, Linux, Mac or truly any operating system. There will always be some variation with Windows as binaries end with .exe and they have no equivalant to rsync other than sFTP. Otherwise this software this software should be completely cross-platform. 

With Golang's concurrency, this will greatly outrun and perform faster than other DevOps software. In the event it is too fast, one can slow it down with the -t or --timeout flags. So you control the speed as you need it.

The biggest advantage of this vs other DevOps software is that you are not locked into having to script with only yaml or ruby. You can ANY programming or scripting langauge you wish here. If you want, you can use Bash, Powershell, Python, Perl, C or whatever you wish. So this will give the user more freedom to use what they are comfortable with and/or use better tools for specific jobs. 

You can use --help for most of the usage. 

$ godev --help<br>
Usage of godev:<br>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;-f, --file string       File containing commands (default "commands.txt")<br>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;-h, --host string       Single IP address or hostname<br>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;-i, --inventory string  Path to inventory file (must start with "inventory")<br>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;-w, --password          Prompt for SSH password<br>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;-p, --port int          SSH port (default 22)<br>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;-s, --script string     Path to a script or binary to upload and execute<br>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;-t, --timeout int       Timeout in seconds for SSH connection (e.g., 10)<br>
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;-u, --user string       SSH username<br>

# Todo
Keep checking semgrep.<br>
Refactor for performance.<br>
Ponder making WinSync part of the rest of the code.<br>
Create installer script and polish README.<br>
