# Requirements

## SSH access

In order to read logs from a host, one has to have ssh access to that host (except for `localhost`); and obviously have read access to the log files. Notably, to read `/var/log/syslog` or `/var/log/messages`, one typically has to be added to the `adm` group.

See the consequent limitations, and possible workarounds, below.

## SSH Public key authentication

Public keys are the only way to SSH-authenticate; preferably via `ssh-agent`, but using the keys directly is also supported (and if the key is protected by the passphrase, Nerdlog will ask for it).

Password SSH authentication is not supported.

## Host requirements

Nerdlog agent relies on a bunch of standard tools to be present on the hosts, such as `bash`, `awk`, `tail`, `head`, `gzip` etc; many systems will already have everything installed, but a few special requirements are worth mentioning:

  * Gawk (GNU awk) is a requirement, since nerlog relies on the `-b` option, to treat the data as bytes, not chars. Technically could be worked around, but will be significantly slower on big log files (slower not because awk is slower without `-b`, but because we'll have to deal with the line numbers instead of byte offsets everywhere, and when we're querying a certain timeframe, it's much more effective to say "get the last 10000000 bytes from this file" instead of "get the last 100000 lines from that file"). So notably, `mawk` will not work. You need `gawk`.
  * A bunch of timestamp formats are supported, and more can be added, but the primary limitation so far is that timestamp must be the first thing in every log line (or at the very least, every component of the timestamp should be at a stable offset from the beginning of the line).
