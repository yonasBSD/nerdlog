# Core concepts

## Logstreams

As the name suggests, a logstream (or shortened to `lstream`) is a consecutive stream of log messages; in Nerdlog implementation, two kinds of logstreams are supported:

  * Provided by either one or more log files (actually, as of now the limitation is to have at most 2 files in a logstream, but this limitation will hopefully be removed at some point). For example, `/var/log/syslog.1` and `/var/log/syslog` constitute a single logstream;
  * Provided by `journalctl`.

By default, nerdlog checks available logstreams in the following order:

  * If available, use the `/var/log/messages` file (and the older `/var/log/messages.1`)
  * If available, use the `/var/log/syslog` file (and the older `/var/log/syslog.1`)
  * As the last resort, use `journalctl` if available.

Why preferring the logfiles instead of `journalctl`: because [log files work much faster and are more reliable](https://github.com/dimonomid/nerdlog/issues/7#issuecomment-2820521885). For some benchmarks, [also see this comment](https://github.com/dimonomid/nerdlog/issues/7#issuecomment-2823303380).  However, `journalctl` is more universally available these days, and it often also has longer log history, so nerdlog has full support for it.

In order to collect data from a logstream, Nerdlog needs to know a few things: first the ssh connection details (hostname, user and port), filename of the last log file, and filename of the previous log file (in the future there might be support for more older files). In the most explicit form, here's how a single logstream specification would look like:

```
myuser@myhost.com:22:/var/log/syslog:/var/log/syslog.1
```

It's a valid syntax and can be used on the query edit form. Multiple logstreams can be provided too, comma-separated.

If you want to select `journalctl` explicitly, specify `journalctl` as the "file":

```
myuser@myhost.com:22:journalctl
```

However, having many hosts to connect to, it would be tedious having to specify them all like that; so, here's how it can be simplified:

### Default values

Everything except hostname is optional here: just like you'd expect, user defaults to the current OS user, and port defaults to 22. Then, as mentioned above, latest logfile defaults to either `/var/log/messages` or `/var/log/syslog` or `journalctl` (whatever is present on the host), and the previous log file defaults to the same as latest one but with the appended `.1` to it, so e.g. `/var/log/syslog.1`, just like log rotation tools normally do (irrelevant for `journalctl`, obviously).

Putting it all together, if the defaults work for us, all we have to do is to specify `myhost.com`. Or again, multiple hosts like `foo.com,bar.com`.

### SSH config

Nerdlog reads ssh config file (`~/.ssh/config`) as well. So for example if our ssh config contains this:

```
Host myhost-01
  User myuser
  HostName actualhost1.com
  Port 1234

Host myhost-02
  User myuser
  HostName actualhost2.com
  Port 7890
```

Then we can specify just `myhost-01`, and it'll be an equivalent of:

```
myuser@actualhost1.com:1234
```

Globs are supported too, so if we want to get logs from both hosts in this ssh config, we can specify `myhost-*`, and it'll be an equivalent of:

```
myuser@actualhost1.com:1234,myuser@actualhost2.com:7890
```

Keep in mind though that as of today, ssh config parsing unfortunately has some major limitations; check out [this issue on Github](https://github.com/dimonomid/nerdlog/issues/12) for more details.

### Nerdlog logstreams config

One obvious problem though is that SSH config doesn't let us specify the log files to read. If we need to configure non-default log files, we can use the `~/.config/nerdlog/logstreams.yaml` file, which looks like that:

```
log_streams:
  myhost-01:
    hostname: actualhost1.com
    port: 1234
    user: myuser
    log_files:
      - /some/custom/logfile
  myhost-02:
    hostname: actualhost2.com
    port: 7890
    user: myuser
    log_files:
      - /some/custom/logfile
```

Having that, we can specify `myhost-01`, and it'll be an equivalent of:

```
myuser@actualhost1.com:1234:/some/custom/logfile:/some/custom/logfile.1
```

### Combining multiple configs

In fact, Nerdlog checks all of these configs in the following order, where every next step can fill missing things in, using hostname as a key:

  * Specified logstream
  * Nerdlog config
  * SSH config
  * Defaults

Therefore, having the SSH config as shown above, we can simplify the aforementioned `logstreams.yaml` as follows:

```
log_streams:
  myhost-01:
    log_files:
        * /some/custom/logfile
  myhost-02:
    log_files:
        * /some/custom/logfile
```

And get the same result, because hostname, user and port will come from the SSH config.

### Reading log files with sudo

It is obviously a security risk, so think twice. Using `journalctl` might be a better option.

On some systems like OpenSUSE, `/var/log/messages` is owned by `root:root`, so one has to either login as root, or use `sudo`.

If you want to use `sudo`, then first of all, make sure that the password is not required for your user, and then add `options: {"sudo": true}` to `/etc/logstreams.yaml` for the corresponding logstream, like this:

```
log_streams:
  myhost-01:
    # ... Potentially any other configuration for the logstream
    options: {"sudo": true}
```

A note on security: allowing sudo without a password is of course a massive security issue.

To make it more secure, it's technically possible to provision the host(s) by uploading that agent script manually under e.g. `/usr/local/bin`, owned by root, and then make it possible in Nerdlog to use that script instead of uploading a new one every time. It's not yet supported in Nerdlog, since manual provisioning like that means some maintenance burden every time nerdlog is updated, or every time we need to read logs from a new host, so I'm not sure if it's worth. Let me know if you actually need it for your use case, and I can hopefully make it happen.

## Query

A Nerdlog query consists of 3 primary components and 1 extra:

  * Logstreams to connect to: where to get the logs from;
  * Time range to read;
  * Optional awk pattern: to filter the logs in the selected time range.

On the query edit form, you'll see one more field: "Select field expression", it looks like this:

```
time STICKY, lstream, message, *
```

But it only affects the presentation of the logs in the UI. It somewhat resembles the SQL `SELECT` syntax, although a lot more limited.

The `STICKY` here just means that when the table is scrolled to the right, these sticky columns will remain visible at the left side.

Another supported keyword here is `AS`, so e.g. `message AS msg` is a valid syntax.
