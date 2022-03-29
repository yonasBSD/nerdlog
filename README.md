
## TODO first

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
- Implement the rest of QueryEditView properly (time, query, all labels etc)
- Fix the bug that after a single time parsing error in the command line, other
  errors stop showing
- Implement profile id, combined from the OS user plus some optional custom string
  given as a command line flag
- Implement state persistence (in a directory with the name based on profile id
  above)
- Implement splitting context from log messages (and also retain the original msg,
  which should be shown when the user presses Enter)
- Implement better indication of the ongoing query (idk how to do that yet)

## TODO next

- :w /path/to/file
- Inputs with history (use for all inputs: command line, query line, all the
  Edit form fields), with state being stored somewhere under profile dir
- Loading next portion of logs
- Proper shutdown, with connections being terminated
- Visual time selection on the histogram

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

### Super important

- Config (and have one ready with all our hosts)
- Changing sources where to query from

-----

- Showing full original raw message in a messagebox
- Get tags out of the message
