## TODO before publishing

- Reimplement host selection (see details below)
- Reimplement message parsing (see details below)
- Implement some fake logs generation, just to use in examples
- Write docs

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

### Reimplement message parsing

It should be customizable, one way or the other.

The simplest is to just factor out some Go abstraction, so that we could have
multiple implementations, and in the config file we'll be able to select which
one to use.

But maybe we can actually make it scriptable somehow.

Need to check what https://lnav.org/ uses, maybe get some ideas from there.

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

## TODO next

- Inputs with history (use for all inputs: command line, query line, all the
  Edit form fields), with state being stored somewhere under profile dir
- Proper shutdown, with connections being terminated

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
