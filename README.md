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

## UI

UI consists of a few key elements:

- Query input: just a filter for logs (awk syntax). Empty filter obviously means no filter, and some examples of valid filters are:
  - Simple regexp: `/foo bar/`
  - Regexps with complex conditions: `( /foo bar/ || /other stuff/ ) && !/baz/`
- Edit button: opens a complete query edit form: it contains the same query input as in the main window, but it also has inputs for hosts filter and time range. The form itself has enough details described right there so you won't have problems using it:

  ![Query Edit Form](images/nerdlog_query_edit_form.png)

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
  - Green: number of hosts which we're fully connected to and which are idle
  - Orange: number of hosts which we're fully connected to and which are executing a query
  - Red: number of hosts which we're trying to connect to
  - Gray: number of hosts found in the config but filtered out by the current hosts filter

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

In the query edit form (the Edit button on the UI, or the `:e[dit]` command), the `Ctrl+K` / `Ctrl+J` iterates "full" query history (affecting not only one field like query, but all of them: time range, hosts filter, query).

## Commands

In addition to the UI which is self-discoverable, there is a vim-like command line
with a few commands supported.

`:xc[lip]` Copies to clipboard a command string which would open nerdlog with
the current hosts filter, time range and query. This can be done from the Menu too (Menu -> Copy query command)

This is the equivalent of URL sharing for web-based logging tools: when you'd
normally copy the graylog URL and paste it in slack somewhere, with nerdlog you
can do the same by sharing this string.

The string would look like this:

```
nerdlog --hosts 'my-node*' --time -3h --query '/something/'
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

- `numlines`: the number of log messages loaded from every host on every
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
