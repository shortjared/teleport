# Admin Manual

This manual covers installation and configuration of Teleport and the ongoing 
management of a Teleport cluster. It assumes that the reader has good understanding 
of Linux administration.

## Installation

### Installing from Source

Gravitational Teleport is written in Go language. It requires Golang v1.5 or newer. 
If you have Go already installed, type:

```bash
> git clone https://github.com/gravitational/teleport 
> cd teleport
> make 
```

If you do not have Go but you have Docker installed and running, you can build Teleport
this way:

```bash
> git clone https://github.com/gravitational/teleport
> cd teleport/build.assets
> make
```

### Installing from Binaries

You can download binaries from [Github](https://github.com/gravitational/teleport/releases). 

## Running

Teleport supports only a handful of commands

|Command     | Description
|------------|-------------------------------------------------------
|start       | Starts the Teleport daemon.
|configure   | Dumps a sample configuration file in YAML format into standard output.
|version     | Shows the Teleport version.
|status      | Shows the status of a Teleport connection. This command is only available from inside of an active SSH seession.
|help        | Shows help.

When experimenting you can quickly start `teleport` with verbose logging by typing 
`teleport start -d`. In production, we recommend starting teleport daemon via an 
init system like `systemd`.  Here's the example of a systemd unit file:

```
[Unit]
Description=Teleport SSH Service
After=network.target 

[Service]
Type=simple
Restart=always
ExecStart=/usr/bin/teleport --config=/etc/teleport.yaml start

[Install]
WantedBy=multi-user.target
```

## Configuration

You should use a configuration file to configure the `teleport` daemon. 
But for simpler experimentation you can use command line flags to 
`teleport start` command. To see the list of flags:

```
> teleport start --help
```

Output:

```
usage: teleport start [<flags>]
Flags:
  -d, --debug         Enable verbose logging to stderr
  -r, --roles         Comma-separated list of roles to start with [proxy,node,auth]
      --advertise-ip  IP to advertise to clients if running behind NAT
  -l, --listen-ip     IP address to bind to [0.0.0.0]
      --auth-server   Address of the auth server [127.0.0.1:3025]
      --token         One-time token to register with an auth server [none]
      --nodename      Name of this node, defaults to hostname
  -c, --config        Path to a configuration file [/etc/teleport.yaml]
      --labels        List of labels for this node
```

### Configuration Flags

Lets cover some of these flags in more detail:

* `--roles` flag tells Teleport which services to start. It is a comma-separated
  list of roles. The possible values are `auth`, `node` and `proxy`. The default 
  value is `auth,node,proxy`. These roles are explained in the 
  [Teleport Architecture](architecture.md) document.

* `--advertise-ip` flag can be used when Teleport nodes are running behind NAT and
  their externally routable IP cannot be automatically determined.

* `--nodename` flag lets you assign an alternative name the node which can be used
  by clients to login. By default it's equal to the value returned by `hostname` 
  command.

* `--listen-ip` shoud be used to tell `teleport` daemon to bind to a specific network
  interface. By default it listens on all.

* `--labels` flag allows to assign a set of labels to a node. See the explanation
  of labeling mechanism in "Labeling Nodes" section below.
  
### Configuration File

Teleport uses YAML file format for configuration. A sample configuration file is shown
below. By default it is stored in `/etc/teleport.yaml`

**WARNING:** When editing YAML configuration, please pay attention to how your editor 
             handles white space. YAML requires consistent handling of tab characters.

```yaml
# By default, this file should be stored in /etc/teleport.yaml

# This section of the confniguration file applies to all teleport
# services.
teleport:
    # nodename allows to assign an alternative name this node can be reached by.
    # by default it's equal to hostname
    nodename: graviton

    # one-time invitation token used to join a cluster. it is not used on 
    # subsequent starts
    auth_token: xxxx-token-xxxx

    # list of auth servers in a cluster. you will have more than one auth server
    # if you configure teleport auth to run in HA configuration
    auth_servers: 10.1.0.5:3025, 10.1.0.6:3025

    # Teleport throttles all connections to avoid abuse. These settings allow
    # you to adjust the default limits
    connection_limits:
        max_connections: 1000
        max_users: 250

    # Logging configuration. Possible output values are 'stdout', 'stderr' and 
    # 'syslog'. Possible severity values are INFO, WARN and ERROR (default).
    log:
        output: stderr
        severity: ERROR

    # Type of storage used for keys. You need to configure this to use etcd
    # backend if you want to run Teleport in HA configuration.
    storage:
        type: bolt
        data_dir: /var/lib/teleport

# This section configures the 'auth service':
auth_service:
    enabled: yes
    listen_addr: 127.0.0.1:3025

# This section configures the 'node service':
ssh_service:
    enabled: yes
    listen_addr: 127.0.0.1:3022
    # See explanation of labels in "Labeling Nodes" section below
    labels:
        role: master
        type: postgres
    # See explanation of commands in "Labeling Nodes" section below
    commands:
    - name: hostname
      command: [/usr/bin/hostname]
      period: 1m0s
    - name: arch
      command: [/usr/bin/uname, -p]
      period: 1h0m0s

# This section configures the 'proxy servie'
proxy_service:
    enabled: yes
    listen_addr: 127.0.0.1:3023
    web_listen_addr: 127.0.0.1:3080

    # TLS certificate for the server-side HTTPS connection.
    # Configuring these properly is critical for Teleport security.
    https_key_file: /etc/teleport/teleport.key
    https_cert_file: /etc/teleport/teleport.crt
```

## Adding and Deleting Users

A user identity in Teleport exists in the scope of a cluster. The member nodes
of a cluster have multiple OS users on them. A Teleport administrator can assign
"user mappings" to every Teleport account, allowing it to login as one of the 
specified OS users.

Lets look at this table:

|Teleport Username | User Mappings | Description
|------------------|---------------|-----------------------------
|joe    | joe,root | Teleport user 'joe' can login into member nodes as OS user 'joe' or 'root'
|bob    | bob      | Teleport user 'bob' can login into member nodes only as OS user 'bob'
|ross   |          | If a mapping is not specified, it defaults to the same name as the Teleport user.

To add a new user to Teleport you have to use `tctl` tool on the same node where
the auth server is running, i.e. `teleport` was started with `--roles=auth`. 

```bash
> tctl users add joe joe,root
```

Teleport generates an auto-expiring token (with a TTL of 1 hour) and prints the token 
URL which must be shared with a user beforeo the TTL expires. 

```
Signup token has been created. Share this URL with the user:
https://<proxy>:3080/web/newuser/xxxxxxxxxxxx

NOTE: make sure the <proxy> host is accessible.
```

The user will complete registration by visiting this URL, picking a password and 
configuring the 2nd factor authentication. After that the account will become 
visible via `tctl`:

```bash
> tctl users ls

User           Allowed to Login as
----           -------------------
admin          admin,root
ross           ross
joe            joe,root 
```

Joe would need to use the `tsh` client tool to login into member node "luna" via 
bastion "work" _as root_:

```bash
> tsh --proxy=work --user=joe root@luna
```

To delete this user:

```bash
> tctl users del joe
```

## Controlling access

At the moment `teleport` does not have a command for modifying an existing user record.
The only recipe to update user mappings or reset user password is to remove the account
and re-create it. 

The user will have to re-initialize Google Authenticator on their phone.

## Adding nodes to the cluster

Gravitational Teleport is a cluster SSH manager. It only allows SSH access to nodes
who had been previously granted cluster membership.

Use `tctl` tool to "invite" a new node to the Teleport cluster:

```bash
tctl nodes add
```

Just like with adding users above, Teleport generates a single-use auto-expiring token
with a TTL of 15 minutes and prints the following:

```
The invite token: n7305ee47a3829e118a7466ac7a0d78ad
Run this on the new node to join the cluster:
> teleport start --roles=node --token=n7305ee47a3829e118a7466ac7a0d78ad --auth-server=<Address>
```

`tctl` is kind enough to even show you the exact command you would need to use on the
new member node to start a `teleport` node service on it.

When a new node comes online, it will start sending ping requests every few seconds
to the auth server. This allows everyone to see which nodes are up:

```bash
> tctl nodes ls

Node Name     Node ID                                  Address            Labels
---------     -------                                  -------            ------
turing        d52527f9-b260-41d0-bb5a-e23b0cfe0f8f     10.1.0.5:3022      distro:ubuntu
dijkstra      c9s93fd9-3333-91d3-9999-c9s93fd98f43     10.1.0.6:3022      distro:debian
```

### Labeling Nodes

In addition to specifying a custom nodename, Teleport also allows to apply arbitrary
key:value pairs to each node. They are called labels. There are two kinds of labels:

1. `static labels` never change while the `teleport` process is running. You may want
   to label nodes with their physical location, the Linux distribution, etc.

2. `label commands` or "dynamic labels". Label commands allow you to execute an external
   command on a node at a configurable frequency. The output of that command becomes
   the value of such label. Examples include reporting a kernel version, load averages,
   time after reboot, etc.

Labels can be configured in a configuration file or via `--labels` flag as shown below:

```bash
> teleport start --labels uptime=[1m:"uptime -p"],kernel=[1h:"uname -r"]
```

Obvioiusly the kernel version is not going to change often, so this example runs
`uname` once an hour. When this node starts and reports its labels into the cluster,
users will see:

```bash
> tctl nodes ls

Node Name     Node ID          Address         Labels
---------     -------          -------         ------
turing        d52527f9-b260    10.1.0.5:3022   kernel=3.19.0-56,uptime=up 1 hour, 15 minutes
```

## Troubleshooting

To diagnose problems you can configure `teleport` to run with verbose logging enabled.

**WARNING:** it is not recommended to run Teleport in production with verbose logging
             as it generates substantial amount of data.

Sometimes you may want to reset `teleport` to a clean state. This can be accomplished
by erasing everything under `"data_dir"` directory. Assuming the default location, 
`rm -rf /var/lib/teleport/*` will do.

## Getting Help

Please stop by and say hello in our mailing list: [teleport-dev@xxxxxx.com]() or open
an [issue on Github](https://github.com/gravitational/teleport/issues).

For commercial support, custom features or to try our multi-cluster edition of Teleport,
please reach out to us: `sales@gravitational.com`. 