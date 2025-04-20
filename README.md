# Nerdlog: fast, remote-first, multi-host TUI log viewer with timeline histogram and no central server

Loosely inspired by Graylog/Kibana, but without the bloat. Pretty much no setup
needed, either.

First of all, a little demo. Here, we're dealing with (fake) logs from 4 remote
nodes, and simulating a scenario of drilling down into logs to find the issue,
filtering out irrelevant messages and finding relevant ones.

![Nerdlog](images/nerdlog_demo.gif)

## Project history

It might be useful to know the history to understand the project motivation and
overall direction.

My team and I were working on a service which was printing a fairly sizeable
amount of logs from a distributed cluster of 20+ nodes: about 2-3M log messages
per hour in total. There was no containerization: the nodes were actual AWS
instances running Ubuntu, and our web services were running there directly as
just systemd services, naturally printing logs to `/var/log/syslog`. To read
the logs though, we were using Graylog, and querying those logs for an hour was
taking no more than 1-3 seconds, so it was pretty quick.

Infra people hated Graylog though, since it required some annoying maintenance
from them, and so at some point the decision was made to switch to Splunk
instead. And when Splunk was finally rolled out, I had to find out that it was
incredibly, ridiculously slow. Honestly, looking at it, I don't quite
understand how they are even selling it. If you've used Splunk, you might know
that it has two modes: "Smart" and "Fast". In "Smart" mode, the same query for
an hour of logs was taking _a few minutes_. And in so called "Fast" mode, it
was taking 30-60s (and that "Fast" mode has some other limitations which makes
it a lot less useful). It might have been a misconfiguration of some sort (I'm
not an infra guy so I don't know), but no one knew how or wanted to fix it, and
so it was clear that once Graylog is finally shut down, we'll lose our ability
to query logs quickly, and it was a massive bummer for us.

And I thought that it's just ridiculous. 2-3M log messages doesn't sound like
such a big amount of logs, and it seemed like some old-school shell hacks on
plain log files, without having any centralized logging server, should be able
to be about as fast as Graylog was, and it should be enough for most of our
needs. As you remember, our stuff was running as systemd services printing logs
to `/var/log/syslog`, so these plain log files were readily available to us.
And so that's how the project started: I couldn't stop thinking of it, so I
took a week off, and went on a personal hackathon to implement this
proof-of-concept log fetcher and viewer, which is ssh-ing directly to the
nodes, and analyzing plain log files using `bash` + `tail` + `head` + `awk`
hacks.

It has proven to be very capable of replacing the essential features we had in
Graylog: being fast, querying logs from multiple remote nodes simultaneously,
drawing the histogram for the whole requested time period, supporting context
(key-value pairs) for every message. Apart from that, it was actually refreshing
to use a snappy keyboard-navigated terminal app instead of clunky web UIs, so in
a sense I liked it even more than Graylog. As to Splunk, I ended up almost never
using it to fetch logs from our nodes.

So having that backstory, you can already get a feel of the goals and design of
Nerdlog: it is lazer-focused on being super efficient while querying logs from
multiple remote machines simultaneously, filtering them by time range and
patterns, and apart from showing the actual logs, also drawing the histogram.

## Design highlights

- No cetralized server required; nerdlog establishes an ssh connection to every
  node that the user wants to collect logs from, and keeps them idle in the
  background (although a separate server to store the log files might still
  help in some cases; see the "Requirements and limitations" section below);
- Logs are never downloaded to the local machine in full: all the log filtering
  is done on the remote nodes, and on each query, only the following data is
  downloaded from each node: up to 250 messages from every node (configurable),
  and stats for the histogram. It's obviously possible to paginate over the
  logs, to get the next bunch of messages, etc. Nerdlog then merges the
  responses from all nodes together, and presents to the user in a unified
  form;
- Most of the data is gzipped in transit, thus saving the bandwidth as well.

## Project state

The initial implementation (that personal hackathon I mentioned above) took
place in 2022, and after reaching the good-enough point, pretty much no
development was done.

It was good enough for our internal needs, but definitely not good enough for
publishing it, and so after a few years, I finally made an effort in 2025 to
address the most obvious issues, and share it.

It's still kinda in a proof-of-concept stage though. Implemented as fast as
possible, spaghetti code abounds, could be covered with more tests, a lot more
features could be implemented, etc. Was only tested for real on Linux, and with
Linux hosts to get the logs from.

But it works. It's pretty usable and surprisingly fast.

## Installation

To build it, you need [Go](https://go.dev/). Having it installed:

To install `nerdlog` binary to your `/usr/local/bin`:

```
$ make && make install
```

Or to build and run without installing:

```
$ make && bin/nerdlog
```

## Quick start

Upon first startup, it'll show you a query form

TODO mention localhost and the ssh

## Core concepts

### Logstreams

As the name suggests, a logstream (or shortened to `lstream`) is a consecutive
stream of log messages; in Nerdlog implementation, a logstream can be provided
by one or more log files (actually, as of now the limitation is to have at most
2 files in a logstream). For example, `/var/log/syslog.1` and `/var/log/syslog`
constitute a single logstream.

In order to collect data from a logstream, Nerdlog needs to know a few things:
first the ssh connection details (hostname, user and port), filename of the
last log file, and filename of the previous log file (in the future there might
be support for more older files). In the most explicit form, here's how a
single logstream specification would look like:

```
myuser@myhost.com:22:/var/log/syslog:/var/log/syslog.1
```

It's a valid syntax and can be used on the query edit form. Multiple logstreams
can be provided too, comma-separated.

However, having many hosts to connect to, it would be tedious having to specify
them all like that; so, here's how it can be simplified:

#### Default values

Everything except hostname is optional here: just like you'd expect, user
defaults to the current OS user, and port defaults to 22. Then, latest logfile
defaults to either `/var/log/messages` or `/var/log/syslog` (whatever is
present on the host), and the previous log file defaults to the same as latest
one but with the appended `.1` to it, so e.g. `/var/log/syslog.1`, just like
log rotation tools normally do.

Putting it all together, if the defaults work for us, all we have to do is
to specify `myhost.com`. Or again, multiple hosts like `foo.com,bar.com`.

#### SSH config

Nerdlog reads ssh config file (`~/.ssh/config`) as well. So for example if our
ssh config contains this:

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

Globs are supported too, so if we want to get logs from both hosts in this
ssh config, we can specify `myhost-*`, and it'll be an equivalent of:

```
myuser@actualhost1.com:1234,myuser@actualhost2.com:7890
```

#### Nerdlog logstreams config

One obvious problem though is that SSH config doesn't let us specify the log
files to read. If we need to configure non-default log files, we can use the
`~/.config/nerdlog/logstreams.yaml` file, which looks like that:

```yaml
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

#### Combining multiple configs

In fact, Nerdlog checks all of these configs in the following order, where
every next step can fill missing things in, using hostname as a key:

- Specified logstream
- Nerdlog config
- SSH config
- Defaults

Therefore, having the SSH config as shown above, we can simplify the
aforementioned `logstreams.yaml` as follows:

```yaml
log_streams:
  myhost-01:
    log_files:
      - /some/custom/logfile
  myhost-02:
    log_files:
      - /some/custom/logfile
```

And get the same result, because hostname, user and port will come from the SSH
config.

### Query

A Nerdlog query consists of 3 primary components and 1 extra:

- Logstreams to connect to: where to get the logs from;
- Time range to read;
- Optional awk pattern: to filter the logs in the selected time range.

On the query edit form, you'll see one more field: "Select field expression",
it looks like this:

```
time STICKY, lstream, message, *
```

But it only affects the presentation of the logs in the UI. It somewhat
resembles the SQL `SELECT` syntax, although a lot more limited.

The `STICKY` here just means that when the table is scrolled to the right,
these sticky columns will remain visible at the left side.

Another supported keyword here is `AS`, so e.g. `message AS msg` is a valid
syntax.

### How it works

It might be useful to understand the internal mechanics of it, because certain
usage limitations will be then more obvious.

TODO: elaborate

- Uploads the agent script
- Checks the host env, gets the example log files, detects the timestamp format,
  generates awk expressions
- Indexes log files by minute
- Awk script doing a lot of things in one pass

## Requirements and limitations

### SSH access

In order to read logs from a host, one has to have ssh access to that host (so
far, there is no shortcut even for localhost); and obviously have read access
to the log files. Notably, to read `/var/log/syslog` or `/var/log/messages`,
one typically has to be added to the `adm` group.

For very small startup-ish teams, or personal projects, this shouldn't be an
issue even for production (and that's the primary use case that Nerdlog targets
anyway).

However, if giving ssh access to production hosts to all devs is not cool, and
we still want to use Nerdlog, then there are at least 2 possible ways to
address this issue:

- Sync logs to a remote server, which is only used for logging purposes; and then
  the devs can ssh to that separate server instead of actual production machines.
  `rsyslog` is capable of syncing logs like that and it shouldn't be too much
  of a setup (although I personally haven't used it like that).
- Technically there could be some kind of api served on the nodes so that
  nerdlog would access it, but it'd be a whole new big thing to implement, and
  doesn't really significant advantages over a separate logging server, so it's
  probably not feasible.

### SSH agent

So far, ssh-agent is the only option for Nerdlog to connect to remote nodes;
so the agent should be running and the necessary keys should be added to it
(you can check the keys in the agent by running `ssh-add -L`). You should be
able to connect to the servers without entering a password.

### Uses CPU & IO of the actual hosts

Unlike centralized systems like Graylog or Kibana, Nerdlog fetches the logs
directly from the hosts which generate the logs, and it consumes some CPU and
IO on these hosts to perform the filtering and analysis. So, if the host is
already very overloaded, then getting logs from it might make things worse in
case of an emergency.  Likewise, if the host becomes unresponsive for whatever
reason, we can't get logs from it either.

Just like the previous point, this too can be addressed by syncing logs to a
separate logging server.

### Depends on the log file rotation policy

Typically, there are only 2 log files available (e.g. `/var/log/syslog` and
`/var/log/syslog.1`); there are some more gzipped files (which Nerdlog doesn't
support currently), and typically these files are rotated every day (or every
week in some cases). So unless you make an extra effort to extend the lifetime
of your log files, you'll only be able to read very recent logs.

For our use cases this was totally fine, but mentioning it just in case.

### Host requirements

Nerdlog agent relies on a bunch of standard tools to be present on the hosts,
such as `bash`, `awk`, `tail`, `head`, `gzip` etc; many systems will already
have everything installed, but a few special requirements are worth mentioning:

- Gawk (GNU awk) is a requirement, since nerlog relies on the `-b` option, to
  treat the data as bytes, not chars. Technically could be worked around, but
  will be significantly slower on big log files (slower not because awk is
  slower without `-b`, but because we'll have to deal with the line numbers
  instead of byte offsets everywhere, and when we're querying a certain
  timeframe, it's much more effective to say "get the last 10000000 bytes from
  this file" instead of "get the last 100000 lines from that file"). So notably,
  `mawk` will not work. You need `gawk`.
- If you're going to read system logs (those accessible via `journalctl`), make
  sure that you have `rsyslog` or similar system installed; otherwise, nobody
  is writing to these log files. Notably, on latest Fedora and Debian,
  `rsyslog` is not installed by default.
- A bunch of timestamp formats are supported, and more can be added, but the
  primary limitation so far is that timestamp must be the first thing in every
  log line (or at the very least, every component of the timestamp should be at
  a stable offset from the beginning of the line).

## UI

UI consists of a few key elements:

- Query input: just a filter for logs (awk syntax). Empty filter obviously means no filter, and some examples of valid filters are:
  - Simple regexp: `/foo bar/`
  - Regexps with complex conditions: `( /foo bar/ || /other stuff/ ) && !/baz/`
- Edit button: opens a complete query edit form: it contains the same query input as in the main window, but it also has inputs for logstreams filter and time range. The form itself has enough details described right there so you won't have problems using it:

  ![Query Edit Form](images/nerdlog_agent_edit_form.png)

- Menu button: just opens a menu with a few extra items:

  ![Menu](images/nerdlog_menu_copy.png)

  - Back: Go to the previous query, just like in the browser
  - Forward: Go to the next query, just like in the browser
  - Copy query command: It's the equivalent of copying an URL in the browser, containing the link to the current logs query. See the `:xc[lip]` command below for more details on that.

- Time range histogram: similarly to some web-based log viewers, like Graylog or Kibana, Nerdlog also shows a timeline histogram, so you can quickly glance at the intensiveness of the logs accordingly to the current query. It's also easy to visually select and apply timerange (using arrow / PgUp / PgDown / Home / End / Enter keys or vim-like bindings):

  ![Timerange Select](images/nerdlog_timerange_select.gif)

- Logs table: obviously contains the actual logs. Like in the normal, old-school logs, **the latest message is on the bottom**. I don't know why modern web tools do it the other way around (latest message being on the top), to me it's nonsense. But let me know if you prefer it this modern way; it's easy to make it configurable.

  Every line shows the timestamp and the message, and it can also be scrolled to the right to show the context tags parsed from a log line.

- Status line. On the left side, there are a few computer icons with numbers:
  - Green: number of lstreams which we're fully connected to and which are idle
  - Orange: number of lstreams which we're fully connected to and which are executing a query
  - Red: number of lstreams which we're trying to connect to
  - Gray: number of lstreams found in the config but filtered out by the current logstreams filter

  And on the right side, there are 3 numbers like `1201 / 1455 / 2948122`. The rightmost number (2948122) is the total number of log messages that matched the query and the timerange (and included in the timeline histogram above). The next number (1455) is the number of actual log lines currently loaded in the nerdlog app, and the leftmost (1201) is just the cursor within those available logs.

- Command line: Vim-like command line. Hit `:` to enter command mode.

## Navigation

There are multiple ways to navigate the app, and you can mix them as you wish.

The most conventional one is to just use Tab and Shift+Tab to switch between widgets (logs table, query input, Edit and Menu buttons, timeline histogram), arrows and keys like Home / End / PgUp / PgDn to move around within a widget, Enter to apply things, Escape to cancel things.

If you know Vim though, you'll feel right at home in nerdlog too since it supports a bunch of Vim-like keybindings:

- Keys `h`, `j`, `k`, `l`, `g`, `G`, `Ctrl+U`, `Ctrl+D`, etc move cursor whenever you're not in some text-editing field, like query input or others
- Hitting Escape eventually brings you to the "Normal mode", which means that the logs table is focused (and all of those `h`, `j`, `k`, `l`, etc work there)
- `:` focuses the command line where you can input some commands (see below)
- `i` or `a` focuses the main query input field

When in an input field (command line, query input, etc), you can go through input history using `Up` / `Down` or `Ctrl+P` / `Ctrl+N`.

In the query edit form (the Edit button on the UI, or the `:e[dit]` command), the `Ctrl+K` / `Ctrl+J` iterates "full" query history (affecting not only one field like query, but all of them: time range, logstreams filter, query).

## Commands

In addition to the UI which is self-discoverable, there is a vim-like command line
with a few commands supported.

`:xc[lip]` Copies to clipboard a command string which would open nerdlog with
the current logstreams filter, time range and query. This can be done from the Menu too (Menu -> Copy query command)

This is the equivalent of URL sharing for web-based logging tools: when you'd
normally copy the graylog URL and paste it in slack somewhere, with nerdlog you
can do the same by sharing this string.

The string would look like this:

```
nerdlog --lstreams 'localhost' --time -3h --pattern '/something/'
```

And it can be used in either the shell (which would open a new instance of
nerdlog), OR it can also be used in a currently running nerdlog instance: just
type `:` to go to the command mode, copypaste this command above, and nerdlog
will parse it and apply the query.

`:back` or `:prev` Go to the previous query, just like in the browser. This can be done from the Menu too (Menu -> Back)

`:fwd` or `:next` Go to the next query, just like in the browser. This can be done from the Menu too (Menu -> Forward)

`:e[dit]` Open query edit form; you can do the same if you just use Tab to navigate
to the Edit button in the UI.

`:w[rite] [filename]` Write all currently loaded log lines to the filename.
If filename is omitted, `/tmp/last_nerdlog` is used.

`:set option=value` Set option to the new value

`:set option?` Get current value of an option

Currently supported options are:

- `numlines`: the number of log messages loaded from every logstream on every
  request. Default: 250.

`:q[uit]` Quit the app.

## Commands

TODO

## Options

TODO

## FAQ

### Why the patterns are in awk syntax?

Because performance, TODO describe

### How is it better than lnav?

It's not better, and not worse. It's just very different.

Lnav's primary focus is to work with local log files, and it's great at it. You
can just throw the whole directory with logs at lnav, and it'll find its way.
It's possible to read remote logs as well, but it was never lnav's primary
focus, and so remains an extra feature on top. For example, it's not practical
to use lnav to check logs from 20+ nodes with 500MB log files each.

Nerdlog's primary focus is to work with remote logs, and to be efficient at it
even when log files are large. Yes you can absolutely read logs from 20+ nodes
with 500MB log files each.

