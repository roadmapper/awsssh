# awsssh

A tool to SSH into EC2 instances via an appropriate bastion<sup>[1](#bastions)</sup>. Inspired by work at previous companies where a custom bastion service was required to log into instances.

[AWS SSM](https://aws.amazon.com/blogs/infrastructure-and-automation/toward-a-bastion-less-world/) is a good alternative to doing something like this

## Installation

### From Source
If you have go 1.17.x installed:
```bash
go build
```

### From GitHub Enterprise
Binaries for macOS, Windows, and Linux are published [here](https://github/roadmapper/awsssh/releases).

[comment]: <> (This won't work due to the way this tap is set up, TODO integrate with a homegrown tap)
[comment]: <> (### From Homebrew)
[comment]: <> (Available on via the `roadmapper/homebrew` tap:)
[comment]: <> (```)
[comment]: <> (brew tap roadmapper/homebrew https://github.com/roadmapper/homebrew.git)
[comment]: <> (brew install awsssh)
[comment]: <> (```)

#### Manual Installation

1. On the right hand side of this GitHub page, go to "Releases"
2. Select/Download the `.tar.gz` file for your operating system (i.e. Darwin for macOS)
3. Open a terminal
4. Move into the directory where it downloaded the file (ie. ~/Downloads)
   - `cd <DIRECTORY>`
5. Extract the `.tar.gz` file
   - `tar -xf <FILENAME>.tar.gz`
6. Make your binary avialble
   - *macOS/Linux*: Move binary to `mv awsssh /usr/local/bin` **OR** Add the location of the binary to your PATH
   - *Windows*: Whereever you put the binary, add that directory to your PATH

**NOTE:** On macOS, you may get a prompt that the binary developer cannot be verified.  In that case, Go to Security & Privacy and add the binary to Developer Tools

## Usage

### SSH into instances with a known IP/hostname
Hostname DNS resolution is supported, if you have an instance behind an A record and don't want to `nslookup` the IP address every time:
```
awsssh example.com
```
This will expand into:
```
ssh <username>@<ipaddress>
```

### SSH into instances by searching AWS
For querying by the `Name` tag on your EC2 instances, the tool assumes your AWS profiles and security tokens have been configured for your user:

The tool will read your `$HOME/.aws/credentials` file to determine what profiles you have access to before querying the EC2 API.

Search for an instance tag name:
```
awsssh -q myinstance
```

Search for an instance ID:
```
awsssh -i i-0123456789abcdefg
```

Search for instances by region:
```
awsssh --region us-west-2
```

Usage:
```
NAME:
   awsssh - SSH into AWS EC2 instances

USAGE:
   awsssh [global options] command [command options] [arguments...]

VERSION:
   1.0.9

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --profile value             the AWS profile to use when connecting [$AWS_DEFAULT_PROFILE]
   --region value, -r value    AWS region (default: "us-east-1") [$AWS_DEFAULT_REGION]
   --query value, -q value     query that will search by the name tag
   --instance value, -i value  instance ID
   --help, -h                  show help
   --version, -v               print the version
```

## Releasing
```bash
make
VERSION=<version> make version
git push --tags origin master
make publish
```

## Notes
<a name="bastions">1</a>: A reference architecture from AWS on how bastions are typically configured: https://aws.amazon.com/quickstart/architecture/linux-bastion/