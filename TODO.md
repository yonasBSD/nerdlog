
## TODO first

- :set numlines=100
- Use https://github.com/benhoyt/goawk just to check the awk syntax before
  submitting the query
- Implement hosts filtering:
  - Use https://github.com/benhoyt/goawk and have filters be defined like
    "/my-host-/", or "/my-host-/ && !/01/ && !/02/"
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
