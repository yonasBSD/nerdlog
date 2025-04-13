## TODO before publishing

- Caption when histogram is focused should try hard to avoid messing with the
  cursor: be on the left or the right, but don't close the cursor whenever possible.
  As it is now it looks bad on short periods like 1h
- BUG: Try to use a smaller window with my usual 4 logstreams: somehow the
  status line looks broken, with the text way at the left and no computer icons
- Implement parsing of the new syslog format: instead of "Apr  5 11:07:46",
  it can be "2025-04-05T11:07:46.161001+03:00" if `/etc/rsyslog.conf`
  doesn't contain `$ActionFileDefaultTemplate RSYSLOG_TraditionalFileFormat`
- Handle corner cases like only having a single log file, or the old one is empty
- Reimplement logstream selection (see details below)
  - Aliases to be used as "source"
  - Parsing of things like {foo,bar}: so that
    "myserver.com:/var/log/{kern,auth}.log" becomes
    "myserver.com:/var/log/kern.log, myserver.com:/var/log/auth.log"
  - Config file with a list of predefined stuff
- Reimplement message parsing using Lua. Default implementation for syslog
  should fetch the app part of every syslog message.
- Implement some fake logs generation, just to use in examples
- Write docs
- In histogram navigation, try to support Ctrl+Left and Ctrl+Right
- Ideally, during node agent initialization, we should check the tools and their
  versions available on the logstream (such as awk), and error out if they're too old
  or otherwise incompatible.
- Ideally, if ssh-agent doesn't have the key, try to add it and request
  passphrase interactively (e.g. ssh itself is able to figure which key to open
  and to add to ssh agent)
- Add support for days in relative timestamps, e.g. "4d"
- With relative timestamps, snap these to the grid. So the snapping logic
  should be unified for these rel. timestamps, xMarks, and bin sizes.
- When histogram is created or new data is fed, the cursor for some reason is not
  at the very end
- Setting timezone should support strings like UTC+3 or UTC-1
- In search bar and command line, Ctrl+P should only use commands starting from
  the current input
- Would be nice to have UI with all the options and their values, and a way to
  change them
- Would be nice to implement a non-ssh way to access localhost, just so that
  the default query of localhost works right away for everyone.
- In addition to MOAR on the top, might also have something like NEWER on the
  bottom. Or maybe start with just REFRESH, and think about the NEWER later.
- Perhaps rename MOAR to OLDER
- Maybe, when scrolling the logs, add an indication on the histogram of where
  on the timeline the current log cursor is. Maybe with just the color (which
  means we can't do half-char width thing, but good enough)
- Favorite queries: I'm def enjoying these in myaccounting
- Tidy up logging; at least enable only info logging, not verbose by default.

- During indexing, make sure that the line we're about to print is "larger"
  lexicographically than the previous line. If not, print an error and exit 1;
  this way we'll hopefully be able to detect broken log files
- When nerdlog_agent.sh returns non-zero code, report an error to the user and
  print the last line from stderr
- Add some easy way to inspect the latest response (stdout+stderr). Perhaps simply
  store them in some files under ~/.cache/nerdlog/agent_responses

- Multi-format: there's no need to support specific format in the bash script:
  just pass some arguments about how to get the hh:mm, as well as all the components
  of the time separately, and that's it

- Before recording demo:
- Implement support for full timestamp format in syslog files, to have timestamps
  with millis in the demo
- Implement some spikes in the random log generation, to make it more interesting
  to look at, so we can demo "drilling down" on some log spike, filtering it etc

- There is some issue when the query is in progress and we hit "Reconnect & Retry".
  Easy to reproduce by clicking very fast, while on the query input:
  Enter -> Tab -> Enter.
- General UI issue with modals:
  - Element X is focused
  - Modal A appears, remembers that X was focused, focuses new element AX
  - Modal B appears, remembers that AX was focused, focuses new element BX
  - Modal A disappears, focuses the element X. Problems:
    - That element X is not actually focused, idk why, perhaps because it's
      on a different "page"
    - The modal B stays forever
    - The whole UI gets stuck and has to be restarted
  

### TODO testing

- Benchmarks, with both short and big log files, at least for the most common case
  with --from but no --to
- Try to move the monthByName definitions outside of the function, see if it improves
  performance
- Add support for the new syslog format: we need to change these parts:
    - syslogFieldsToIndexTimestr
    - generation of HHMM, like, "curHHMM = substr($3, 1, 5)": also gotta take
      the HHMM from the right offsets
    - mstats generation: "stats[$1 $2 "-" substr($3,1,5)]++;", which generates
      strings like "Jan2-15:04". Keep in mind that we don't have to make 100%
      unified across all formats: e.g. for the traditional syslog format, it
      makes sense to have this "Jan2" thing here because it's the fastest to
      do in awk; we just need to make sure we know how to parse it for every
      logstream in Go (in `host_agent.go`).
    - hopefully that's all, but maybe there's something that I'm missing
  So a complete format support package would consist of this:
    - bash/awk:
        - syslogFieldsToIndexTimestr and some auxiliary stuff for it, like
          inferYear and monthByName etc
        - generation of HHMM, like "curHHMM = substr($3, 1, 5);"
        - mstats generation, like "stats[$1 $2 "-" substr($3,1,5)]++;"
    - Go:
        - Time layout to parse mstats
        - Some Go function to extract the time from the message
        - A way to parse essentials of the message, like e.g. app name in the
          syslog: either also Go func (perhaps the same as parses the time), OR
          a lua function, which can act on its own or be invoked from a user's
          custom lua function

### Reimplement logstream selection

Reimplement logstream selection: instead of always having to have an exhaustive list
of lstreams to connect to and using globs to filter them, make it so that the user
can specify lstreams without any configuration; in the most verbose form, it'd look
like this: "user@myhost.com:/var/log/syslog:/var/log/syslog.1 as foo" But every part
other than the logstream is optional: default user is the same as the current user,
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

For each logstream that we're specifying in the lstreams input, such as "myhost1, myhost2"
etc, nerdlog needs to figure more details:

- The actual hostname to connect to
- Username
- Port
- Log file

All of these can be specified right in the lstreams input, like this:

```
myuser@myhost:1234:/path/to/logfile
```

And, provided that `myhost` is the actual hostname to connect to, then nerdlog
has everything it needs for this particular logstream.

If however one or more parts was omitted, then, for each missing part, it will
try to find it in the following places, using the provided logstream (`myhost` in
this case) as a key:

- Nerdlog's own config, located in ~/.config/nerdlog/lstreams
- SSH config ~/.ssh/config (obviously, log file can't be specified there; but
  everything else can)
- Defaults: just like in the ssh itself, the default user is the current user,
  and default port is 22. And the default config is `/var/log/syslog`.

NOTE that in the nerdlog config, would be useful to specify MULTIPLE default
log files, BUT it kinda adds ambiguity: having e.g. 2 log files for myhost, if
we specify just "myhost", it's logical that we'll make 2 conns to that logstream, with
different files. BUT what if we specify "myhost, myhost:22:/some/other/log"?
Would the bare "myhost" still expand to the two log files? Hmm I guess so, why not..

## TODO first

- :set numlines=100
- Use https://github.com/benhoyt/goawk just to check the awk syntax before
  submitting the query
- Implement lstreams filtering:
  - Implement logic in logstream manager which will take the filter like that, and
    come up with relevant set of nodes (we'll also need to do that in realtime
    while typing), and reconnect to the new ones etc
- Implement getting all available lstreams from ssh config
  https://github.com/kevinburke/ssh_config , (it probably should be used by
  main, and given to LStreamsManager as the same config). Also implement the nerdlog
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
- Ideally support more than 2 log files
- Allow setting formatting layout for time in the logs

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
  the command like "nerdlog --query foo --lstreams bar --time baz", which is usable
  in the shell AND in the nerdlog command line as well, see:
- :nerdlog ..... : parses the command as if it was the shell command, with flags
  like --query foo etc, and applies those changes

^ this way, it'll be really easy to share the "links" to the logs with each other

- In the query edit form, on the top, have like:
  "History: <prev> <next>"
  And clicking those buttons would go through the history; every history item
  contains all 3 elements: query, lstreams, time; and it should be stored in some
  file like a command line history. When a new request is made, a new line is
  added there.

### Super important

- Config (and have one ready with all our lstreams)
- Changing sources where to query from

-----

- Showing full original raw message in a messagebox
- Get tags out of the message
