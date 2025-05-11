# Limitations

## Consequences of requiring SSH access

As mentioned above, SSH connection is a requirement, which does present a significant limitation as well.

For very small startup-ish teams, or personal projects, this shouldn't be an issue even for production (and that's the primary use case that Nerdlog targets anyway). However, if giving ssh access to production hosts to all devs is not cool, and we still want to use Nerdlog, then there are at least 2 possible ways to address this issue:

  * Sync logs to a remote server, which is only used for logging purposes; and then the devs can ssh to that separate server instead of actual production machines.  `rsyslog` is capable of syncing logs like that and it shouldn't be too much of a setup (although I personally haven't used it like that).
  * Technically there could be some kind of api served on the nodes so that nerdlog would access it, but it'd be a whole new big thing to implement, and doesn't really have significant advantages over a separate logging server, so it's probably not feasible.

## Uses CPU & IO of the actual hosts

Unlike centralized systems like Graylog or Kibana, Nerdlog fetches the logs directly from the hosts which generate the logs, and it consumes some CPU and IO on these hosts to perform the filtering and analysis. So, if the host is already very overloaded in case of an emergency, then getting logs from it might make things worse.  Likewise, if the host becomes unresponsive for whatever reason, we can't get logs from it either.

Just like the previous point, this too can be addressed by syncing logs to a separate logging server, if we consider this problem severe enough.

## Only two last log files in a logstream are supported

A typical configuration for log rotation is to have the current and the previous files in plain text, and a few more older files, usually gzipped. For now, Nerdlog only supports the 2 latest files, such as `/var/log/syslog` and `/var/log/syslog.1`. It can't read `/var/log/syslog.2` etc, regardless if gzipped or not.

It's possible to remove that limitation, and also support unzipping, but for now it's a TODO.

To get older logs, there are 2 workarounds that you can consider:

  * If you're reading system logs, just use `journalctl`: it's slower, but usually has longer history;
  * If you need to read e.g. `/var/log/syslog.2`, then first gunzip it manually, and then specify a logstream like this: `myserver.com:22:/var/log/syslog.1:/var/log/syslog.2`
