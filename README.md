# GoDev
A simple cross-platform DevOps project in Golang that's built for speed and customization. 

Since this is written with golang, you can use this program for Windows, Linux, Mac, Solaris, AIX or truly any operating system. There will always be some variation with Windows as binaries end with .exe and they have no equivalent to rsync other than sFTP. Otherwise this software should be completely cross-platform.

With Golang's concurrency, this will greatly outrun and perform faster than other DevOps software. In the event it is too fast, one can slow it down with the -t or --timeout flags. So you control the speed as you need it.

With this software you are not locked into having to script with only yaml, ruby or some pseudo-code. You can use ANY programming or scripting language you wish here. If you want, you can use Bash, Powershell, Python, Perl, C, Zig, Gleam or whatever you wish if the destination servers can run it. So this will give the user more freedom to use what they are comfortable with and/or use better tools for specific jobs. At this time there is one DevOps tool with this same feature, but it will not match our simplicity or speed. 

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
user@host:port::password:::sudoPassword
```
Like most configuration files the '#' character can be used for comments. Should you have an SSH password, or a sudo password that contains '#', ':', '@' you can escape them with the '\' character. Or if a backslash is used, you can use '\\' to denote a single backslash. Should you have an SSH password like ':@::#:::\', you can escape it like so in an inventory file:
```
10.0.0.3::\:\@\:\:\#\:\:\:\\
```
If we are executing this program with the same user we are logging into the host with, SSH uses keys instead of passwords, SSH is on port 22 and don't need sudo, we could just use the host with nothing else on a line. Once we have configured the inventory file in our current directory we have two options to run code or commands on the hosts we configured. One is to use a .txt file, like commands.txt to include shell commands which we can call from any location with the -f or --file option:
```
$ cat commands.txt 
uname -a
ps fuax
vmstat
echo Works!
```
If we have configured both the inventory and commands.txt file correctly, like so:
```
$ time godev -f /home/user/commands.txt
======================================
----- Output from host 10.0.0.2 -----
======================================

FreeBSD www1.example.com 14.2-RELEASE-p1 FreeBSD 14.2-RELEASE-p1 GENERIC amd64
USER    PID %CPU %MEM   VSZ   RSS TT  STAT STARTED    TIME COMMAND
user 73683  0.0  0.3 24052 10640  -  SJ   02:42   0:00.02 sshd: user@notty (sshd)
user 74970  0.0  0.1 14408  2768  -  SsJ  02:42   0:00.10 sh -c uname -a\nps fuax\nvmstat\necho Works!\n
user 77066  0.0  0.1 14480  2920  -  RJ   02:42   0:00.04 ps fuax
 procs    memory    page                      disks       faults       cpu
 r  b  w  avm  fre  flt  re  pi  po   fr   sr ada0  cd0   in   sy   cs us sy id
 1  0  0 3305889792 3125170176   83   0   2   0   90   20    0    0   19  192  254  0  0 98
Works!

======================================
----- Output from host 10.0.0.3 -----
======================================

FreeBSD www2.example.com 14.2-RELEASE-p1 FreeBSD 14.2-RELEASE-p1 GENERIC amd64
USER    PID %CPU %MEM   VSZ   RSS TT  STAT STARTED    TIME COMMAND
user 74047  0.0  0.3 24052 10648  -  SJ   02:42   0:00.04 sshd: user@notty (sshd)
user 75350  0.0  0.1 14408  2988  -  SsJ  02:42   0:00.09 sh -c uname -a\nps fuax\nvmstat\necho Works!\n
user 77416  0.0  0.1 14480  2924  -  RJ   02:42   0:00.05 ps fuax
 procs    memory    page                      disks       faults       cpu
 r  b  w  avm  fre  flt  re  pi  po   fr   sr ada0  cd0   in   sy   cs us sy id
 0  0  0 3253927936 3127631872   83   0   2   0   91   20    0    0   19  192  254  0  0 98
Works!

        1.16 real         0.29 user         0.08 sys
```
This may vary per environment, but you will probably notice that we ran the program with the time command above and it is greatly faster than other popular DevOps software when running four different tasks across two hosts. If for whatever reason you need to slow this down, you can use the -t or --timeout option to add a pause in a number of seconds between hosts. Godev will always respect the order of commands in the commands.txt file, but it will not necessarily follow the order of hosts in the inventory file. If you need specific actions to happen on specific hosts in a certain order you can configure multiple inventory files and specify them with the -i or --inventory option. The only requirement here is that the file begins with the word "inventory" like inventory_web, inventory_linux, inventory_db, etc. 

There is also another way to run code with the -s or --script option. Using this option we can rsync a script or binary written in any language to the /tmp folder of a host and execute it:

```
$ godev -s ./tests/hello
======================================
----- Output from host 10.0.0.3 -----
======================================

Hello GoDev!

======================================
----- Output from host 10.0.0.2 -----
======================================

Hello GoDev!

```
Of course this will differ slightly in Windows where sFTP is used in place of rsync, but it will accomplish the same goal. 

The only requirements before using on non-Windows hosts are that SSH and rsync be installed and running. Windows 10 and above will only require openssh to be enabled as this will also enable sFTP in the process. To build this software, if golang is installed and you can run the following from inside this project's directory:
```
$ go build .
```
From there you may copy the 'godev' binary from your current folder to /usr/bin or somewhere in PATH. If on Windows this will probably be C:\Windows\System32. For more information on build options, see the INSTALL file.

If you need to run a command in commands.txt with sudo and a password, you can something like the following:
```
echo "P@55w0rd" | sudo -S whoami
```
However, this can be configured more easily in the inventory files. Comments can also be used in commands.txt with '#' as well.

If you need to run an entire script or binary as root, you may just add the sudo password in your inventory file to use with the -s option. Or if no password is used with sudo, you can run it as a normal user to copy it to the /tmp folder on a remote server. Then add something the following to commands.txt and run it this way:
```
sudo /tmp/my_super_script.sh
```
After this project's release, it was pointed out that encryption may be needed for the inventory files. Despite tons of free file encrypters already available, you may now build the code in the 'goenc' folder and use this program to encrypt and decrypt files:
```
$ ./goenc 
Usage:
  -e, --encrypt <path> - Encrypt a file
  -d, --decrypt <path> - Decrypt a file
```
This should conclude any information one needs to know to configure and use this software in all its forms. The fact we have done this in little over 100 lines instead of 100 or more pages like other DevOps software should showcase that simplicity was a goal all along here. 

For bug reports or feature requests, please open an issue at:

https://github.com/mephistolist/godev/issues
