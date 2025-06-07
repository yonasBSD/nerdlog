# Options

Options can be set or checked using `:set` command, for example:

- `:set numlines?` prints the current value of the `numlines` option
- `:set numlines=1000` sets `numlines` to the new value

So far there is no support for a persistent config file where we can specify initial values of these options, but you can provide them using the `--set` flag, like this:

```
$ nerdlog --set 'numlines=1000' --set 'transport=ssh-bin'
```

Currently supported options are:

### `numlines`

The number of log messages loaded from every logstream on every request. Default: 250.

### `timezone`

The timezone to format the timestamps on the UI. By default, `Local` is used, but you can specify `UTC` or `America/New_York` etc.

### `transport`

Specifies what to use to connect to remote hosts (has no effect on `localhost`: this one always goes via local shell).

Valid values are:

- `ssh-lib`: Use internal Go ssh implementation (the [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) library). This is what Nerdlog was using from the day 1, but it's pretty limited in terms of configuration; e.g. if you have more or less advanced ssh configuration, chances are that Nerdlog won't be able to fully parse it. Only some minimal parsing of `~/.ssh/config` is done.
- `ssh-bin`: Use external `ssh` binary. This is still a bit experimental, but a lot more comprehensive. The only observable limitation here is that if the ssh agent is not running, and ssh key is encrypted, then with `ssh-lib` Nerdlog would ask you for the key passphrase, while with `ssh-bin` the connection will just fail.

With `ssh-bin`, Nerdlog also uses the ssh config a bit differently: it only uses the list of hosts parsed from the ssh config to implement globs, so e.g. if your ssh config has two hosts `my-01` and `my-02`, then typing `my-*` in logstreams input would make Nerdlog connect to both of them. But, Nerdlog won't try to figure out the actual hostname, or usename, or port from the ssh config: it would simply run the command like `ssh -o 'BatchMode=yes' my-01 /bin/sh`, leaving all the config parsing up to that `ssh` binary.

However, the Nerdlog's own logstreams config is still interpreted as before; so if in that config you have e.g. this:

```yaml
log_streams:
  my-01:
    hostname: myactualserver.com
    port: 1234
    user: myuser
```

Then the ssh command will actually be: `ssh -p 1234 -o 'BatchMode=yes' myuser@myactualserver.com /bin/sh`

For now, `ssh-lib` is still the default, but the plan is to change that at some point and make `ssh-bin` the default if `ssh` binary is available.
