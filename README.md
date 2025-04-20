# Nerdlog: fast, remote-first, multi-host TUI log viewer with timeline histogram and no central server

Loosely inspired by Graylog/Kibana, but without the bloat. Pretty much no setup
needed, either.

First of all, a little demo. Here, we're dealing with logs from 4 remote nodes,
each having about 2GB log file, generating about 600K log messages per hour in
total.

TODO: implement the demo, and update the numbers above.

![Nerdlog](images/nerdlog.png)

## Project history

It might be useful to know the history to understand the project motivation and
overall direction.

My team and I were working on a service which was printing a fairly sizeable
amount of logs from a distributed cluster of 20+ nodes: about 2-3M log messages
per hour in total. There was no containerization: the nodes were actual AWS
instances running Ubuntu, and our web services were running there directly as
just systemd services, naturally printing logs to `/var/log/syslog`. To read
the logs though, we were using Graylog, and querying those logs for an hour was
taking no more than 1-2 seconds, so it was pretty quick.

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
  node that the user wants to collect logs from, and keeps them idle until a log
  query is made;
- Logs are never downloaded to the local machine in full: all the log filtering
  is done on the remote nodes, and on each query, only the following data is
  downloaded from each node, gzipped: up to 100 messages (configurable), and
  stats for the histogram. It's obviously possible to paginate over the logs, to
  get the next 100 messages, etc. Nerdlog then merges the responses from all
  nodes together, and presents to the user in a unified form.

## Project state

It's still kinda in a proof-of-concept stage. Implemented as fast as possible,
spaghetti code abounds, almost no tests, poor error handling, etc.

But it works. It's pretty usable and surprisingly fast.

## Installation

To install `nerdlog` binary to your `/usr/local/bin`:

```
$ make && make install
```

Or to build and run without installing:

```
$ make && bin/nerdlog
```

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

## Limitations

- SSH access is required, so:
  - might be an issue for prod
  - only for EC2
- Due to current log rotation policy, only 1-2 last days are available
- Uses CPU & IO of the actual nodes, so if the node dies, we can't get logs

^ All of those can be solved by having a separate machine and syncing all log
files to it (just plain log files, nothing fancy).

## How is it different from [lnav](https://lnav.org/)?

It actually differs in a lot of ways. From what I know, lnav was primarily
implemented as a local log viewer (I mean, to read logs on the same machine
where lnav is running, which can obviously be accessed over ssh), and it does a
pretty good job on this. And while the feature to read logs from a remote server
was implemented later on, it's still not possible to e.g. query logs by a time
range and filter them without lnav actually downloading the whole log file,
which might be huge.

Nerdlog, on the other hand, was remote-first from the beginning. It is
lazer-focused on being super efficient while querying logs from multiple remote
machines, filtering them by time range and a pattern, and drawing the chart for
the whole time range.
