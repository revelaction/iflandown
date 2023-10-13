
<p align="center"><img alt="iflandown" src="logo.png"/></p>

## iflandown

`iflandown` is a deamon that runs commands after the wired LAN link is down for a given amount
of time.

All/most linux devices should work. Tested on amd64, Raspberry Pi, odroid 

## Why

You can use `iflandown` to avoid filesystem corruption in your rapsberry
Pis, when they are connected to an Uninterruptible Power Supply (UPS). 
`iflandown` can safely stop the programs running and shutdown the Pi before it
runs beyond the UPS time limit, avoiding thus risk of sd card corruption, in
case of a too long power outage.

For this to work, the home router shouldn't  be protected by the UPS, as it
serves as the detection feature.

## How it works

`iflandown` checks each minute the LAN ethernet interfaces in
`/sys/class/net/%s/carrier` to register the state of the link. 

If a minimum of uptime minutes `Window` is not reached inside a defined period
of time `Period`, `iflandown` executes the configured commands.

## Usage

Edit the `iflandown.toml` file with your preferences:

```
# The past period of time, starting each minute, that is considered to search
# for at least `Window` minutes of LAN (ethernet) uptime 
#
# It is measured in minutes.
#
# iflandown will also wait this time before starting to make decisions.
# 
# This number of minutes should match roughly the capacity of your UPS to
# serve your devices after a power outage.
Period = 30

# The minimum number of minutes in the `Period` of time that are necessary to
# have been "up" in order to avoid the sequence of commands to run. 
Window = 5

# The commands to run
#
# Separate arguments from the main command and include full paths
# 
# sudo commands are possible
Commands = [["ls", "-alrt"], ["sudo", "/bin/systemctl", "stop", "myservice"]]
```

And run the command in the same directory as the `iflandown.toml` file:

```
iflandown
```

To run the commands without checks (to test the commands, permissions), run:

```
iflandown --nocheck
```

## systemd service

There is also a systemd service file to run `iflandown` as a service. 
Just change in the file the user name `<user>`

## Run commands as root without passwords

To let `iflandown` run commands with sudo and no password, add one
line in the `/etc/sudoers` file for each command and arguments that you
configured in `iflandown.toml`.

For example, to let `iflandown` stop the `systemd` service `myservice`,
add this line:

```
    <user> ALL = (root) NOPASSWD: /bin/systemctl stop myservice
```

Where `<user>` is the user that will run the `iflandown` binary/service.

