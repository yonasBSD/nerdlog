# Nerdlog

A proof-of-concept log fetcher and viewer. Features terminal-based UI, works by
ssh-ing directly to the nodes and analyzing syslog files using
`bash` + `tail` + `head` + `awk` hacks.

I said, a proof of concept. Implemented as fast as possible, spaghetti code
abounds, almost no tests, poor error handling, etc.

But it works. It's pretty usable and surprisingly fast.

![Nerdlog](images/nerdlog.png)

## Installation

To install `nerdlog` binary to your `/usr/local/bin`:

```
$ make && make install
```

Or to build and run without installing:

```
$ make && bin/nerdlog
```

## Commands

In addition to the UI which is self-discoverable, there is a vim-like command line
with a few commands supported.

`:xc[lip]` Copies to clipboard a command string which would open nerdlog with
the current hosts filter, time range and query. This can also be done from the
menu in the UI:

![Menu -> Copy query command](images/nerdlog_menu_copy.png)

This is the equivalent of URL sharing for web-based logging tools: when you'd
normally copy the graylog URL and paste it in slack somewhere, with nerdlog you
can do the same by sharing this string.

The string would look like this:

```
nerdlog --hosts 'my-host-*' --time -3h --query '/redacted_symbol_str=kucoin/'
```

And it can be used in either the shell (which would open a new instance of
nerdlog), OR it can also be used in a currently running nerdlog instance: just
type `:` to go to the command mode, copypaste this command above, and nerdlog
will parse it and apply the query.

`:back` or `:prev` Go to the previous query, just like in the browser.

`:fwd` or `:next` Go to the next query, just like in the browser.

`:e[dit]` Open query edit form; you can do the same if you just use Tab to navigate
to the Edit button in the UI.

`:w[rite] [filename]` Write all currently loaded log lines to the filename.
If filename is omitted, `/tmp/last_nerdlog` is used.

`:set option=value` Set option to the new value

`:set option?` Get current value of an option

Currently supported options are:

- `numlines`: the number of log messages loaded from every host on every
  request. Default: 250.

## Limitations

- SSH access is required, so:
  - might be an issue for prod
  - only for EC2
- Due to current log rotation policy, only 1-2 last days are available
- Uses CPU & IO of the actual nodes, so if the node dies, we can't get logs

^ All of those can be solved by having a separate machine and syncing all log
files to it (just plain log files, nothing fancy).
