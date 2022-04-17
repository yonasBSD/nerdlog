
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

### Super important

- Config (and have one ready with all our hosts)
- Changing sources where to query from

-----

- Showing full original raw message in a messagebox
- Get tags out of the message

## Limitations

- SSH access is required, so:
  - might be an issue for prod
  - only for EC2
- Due to current log rotation policy, only 1-2 last days are available
- Uses CPU & IO of the actual nodes, so if the node dies, we can't get logs

^ All of those can be solved by having a separate machine and syncing all log
files to it (just plain log files, nothing fancy).
