# GoDev
A simple cross-platform devops project in Golang that's built for speed and customization. 

Since this is written with golang, you can use this program for Windows, Linux, Mac or truly any operating system. There will always be some variation with Windows as binaries end with .exe and they have no equivalant to rsync other than sFTP. Otherwise this software this software should be completely cross-platform. 

With Golang's concurrency, this will greatly outrun and perform faster than other DevOps software. In the event it is too fast, one can slow it down with the -t or --timeout flags. So you control the speed as you need it.

The biggest advantage of this vs other DevOps software is that you are not locked into having to script with only yaml, ruby or some pseudo-code. You can ANY programming or scripting langauge you wish here. If you want, you can use Bash, Powershell, Python, Perl, C, Zig, Gleam or whatever you wish. So this will give the user more freedom to use what they are comfortable with and/or use better tools for specific jobs. 

You can use --help for most of the usage. 
```
$ godev --help
Usage of godev:
   -f, --file string       File containing commands (default "commands.txt")
   -h, --host string       Single IP address or hostname
   -i, --inventory string  Path to inventory file (must start with "inventory")
   -w, --password          Prompt for SSH password if not using keys
   -p, --port int          SSH port (default 22)
   -s, --script string     Path to a script or binary to upload and execute
   -t, --timeout int       Timeout in seconds for SSH connection (e.g., 10)
   -u, --user string       SSH username
```
Like any DevOps software, most won't see much value until you are using the software across multiple hosts. You can do this by configuring an inventory file which takes the following format:
```
user@host:port::password
```

However, if we are executing this program with the same user we logging into the host with, SSH uses keys instead of passwords and SSH is on port 22, we could just use the host and nothing else on a line.  

# Todo
Keep checking semgrep.<br>
Refactor for performance.<br>
Ponder making WinSync part of the rest of the code.<br>
Create installer script and polish README.<br>
