## TODO before publishing

- Implement parsing of the new syslog format: instead of "Apr  5 11:07:46",
  it can be "2025-04-05T11:07:46.161001+03:00" if `/etc/rsyslog.conf`
  doesn't contain `$ActionFileDefaultTemplate RSYSLOG_TraditionalFileFormat`
- Handle corner cases like only having a single log file, or the old one is empty
- Ideally support more than 2 log files
- Reimplement host selection (see details below)
  - Aliases to be used as "source"
  - Parsing of things like {foo,bar}: so that
    "myserver.com:/var/log/{kern,auth}.log" becomes
    "myserver.com:/var/log/kern.log, myserver.com:/var/log/auth.log"
  - Config file with a list of predefined stuff
- Reimplement message parsing using Lua. Default implementation for syslog
  should fetch the app part of every syslog message.
- Implement some fake logs generation, just to use in examples
- Write docs
- Fix client name (make it include the log filename)
- Fix tmp file naming (make it non-random, so that index is reused between
  invocations)
- Fix an issue with reconnect tight loop
- Fix an issue with things like "[system]" in the log messages being interpreted
  by tview
- Fix the issue with histogram navigation: when the period is 100h, it jumps from
  beginning to end.
- In histogram navigation, try to support Ctrl+Left and Ctrl+Right
- Ideally, during node agent initialization, we should check the tools and their
  versions available on the host (such as awk), and error out if they're too old
  or otherwise incompatible.
- Ideally, if ssh-agent doesn't have the key, try to add it and request
  passphrase interactively (e.g. ssh itself is able to figure which key to open
  and to add to ssh agent)
- In histogram, when selecting, show not only time but also date of the cursor.

### TODO testing

- Benchmarks, with both short and big log files, at least for the most common case
  with --from but no --to
- Try to move the monthByName definitions outside of the function, see if it improves
  performance
- Add support for the new syslog format: we need to change these parts:
    - syslogFieldsToTimestamp needs to detect which format it is, and use it,
      seems easy
    - "curHHMM = substr($3, 1, 5)": also gotta detect which format it is, and
      take the HHMM from the right offsets
    - hopefully that's all, but maybe there's something that I'm missing

### Reimplement host selection

Reimplement host selection: instead of always having to have an exhaustive list
of hosts to connect to and using globs to filter them, make it so that the user
can specify hosts without any configuration; in the most verbose form, it'd look
like this: "user@myhost.com:/var/log/syslog:/var/log/syslog.1 as foo" But every part
other than the host is optional: default user is the same as the current user,
default first log file is `/var/log/syslog`, and default second log file is the
first one with `.1` appended. And if "as <something>" is omitted, obviously it's
just used in `source` as is; otherwise the alias is used.

That's like the bare minimum.

Some nice-to-haves though:

Config file can be used to specify all those details for every particular node,
plus some extras which actually can't be specified inline, like the port number
(or maybe we can make it inline-able, like `myhost.com {"port": 123}`, but idk)

Config file will also make it possible to implement the globs (without the
config, we have nothing to filter from)

---

For each host that we're specifying in the hosts input, such as "myhost1, myhost2"
etc, nerdlog needs to figure more details:

- The actual hostname to connect to
- Username
- Port
- Log file

All of these can be specified right in the hosts input, like this:

```
myuser@myhost:1234:/path/to/logfile
```

And, provided that `myhost` is the actual hostname to connect to, then nerdlog
has everything it needs for this particular host.

If however one or more parts was omitted, then, for each missing part, it will
try to find it in the following places, using the provided host (`myhost` in
this case) as a key:

- Nerdlog's own config, located in ~/.config/nerdlog/hosts
- SSH config ~/.ssh/config (obviously, log file can't be specified there; but
  everything else can)
- Defaults: just like in the ssh itself, the default user is the current user,
  and default port is 22. And the default config is `/var/log/syslog`.

NOTE that in the nerdlog config, would be useful to specify MULTIPLE default
log files, BUT it kinda adds ambiguity: having e.g. 2 log files for myhost, if
we specify just "myhost", it's logical that we'll make 2 conns to that host, with
different files. BUT what if we specify "myhost, myhost:22:/some/other/log"?
Would the bare "myhost" still expand to the two log files? Hmm I guess so, why not..

## TODO first

- :set numlines=100
- Use https://github.com/benhoyt/goawk just to check the awk syntax before
  submitting the query
- Implement hosts filtering:
  - Implement logic in host manager which will take the filter like that, and
    come up with relevant set of nodes (we'll also need to do that in realtime
    while typing), and reconnect to the new ones etc
- Implement getting all available hosts from ssh config
  https://github.com/kevinburke/ssh_config , (it probably should be used by
  main, and given to HostsManager as the same config). Also implement the nerdlog
  own config, where we'll have again another awk filter for nodes which need
  a jumphost, and the jumphost address itself
- Implement state persistence (in a directory with the name based on profile id
  above)

- Add some kind of index format version, so when we change the format in future
  versions, we can automatically rebuild the index without issues

## TODO next

- Inputs with history (use for all inputs: command line, query line, all the
  Edit form fields), with state being stored somewhere under profile dir
- Proper shutdown, with connections being terminated
- Make configurable whether we use case-insensitive pattern matching
  (iirc slows down the query significantly)
- Maybe implement favorites. Just like a button in the top-right corner of the
  Edit dialog, which would show a menu with the favorite queries.

## TODO

- During bootstrap, don't overwrite file if it's up to date
- Use some user-specific name for the resident bash file
  /var/tmp/query_logs.sh, so that multiple users won't be overwriting the same
  file

- Implement configs, since locally I need jumphost but on worker I don't, and also
  right now all nodes are hardcoded

- Statusline: (green) * 21 (orange) * 3 (red) * 0 . When number is non-zero,
  font is bold.
- Searching and highlighting can be implemented using color tags like [:red]foo[:-]

- :xc[lip] [q[uery]|t[ime]|h[osts]] : if no argument is given, it just copies
  the command like "nerdlog --query foo --hosts bar --time baz", which is usable
  in the shell AND in the nerdlog command line as well, see:
- :nerdlog ..... : parses the command as if it was the shell command, with flags
  like --query foo etc, and applies those changes

^ this way, it'll be really easy to share the "links" to the logs with each other

- In the query edit form, on the top, have like:
  "History: <prev> <next>"
  And clicking those buttons would go through the history; every history item
  contains all 3 elements: query, hosts, time; and it should be stored in some
  file like a command line history. When a new request is made, a new line is
  added there.

### Super important

- Config (and have one ready with all our hosts)
- Changing sources where to query from

-----

- Showing full original raw message in a messagebox
- Get tags out of the message
