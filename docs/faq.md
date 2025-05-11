# FAQ

## Why the patterns are in awk syntax?

Because, at least in the current implementation, it's the simplest and most efficient way to implement filtering. As you remember from the "How it works" section above, after cutting the logs outside of the requested time range, we do the filtering, generate timeline histogram data, and print the last N log lines, keeping track of where they were in the original file (so that in the UI we can point the user at that line, if they want to). All this is done using an awk script in a single pass, and obviously it's easier to have filtering as part of the same awk script.

## How is it better than lnav?

It's not better, and not worse. It's just very different.

Lnav's primary focus is to work with local log files, and it's great at it. You can just throw the whole directory with logs at lnav, and it'll find its way.  It's possible to read remote logs as well, but it was never lnav's primary focus, and so remains an extra feature on top. For example, it's not practical to use lnav to check logs from 20+ nodes with 500MB log files each.

Nerdlog's primary focus is to work with remote logs, and to be efficient at it even when log files are large. Yes you can absolutely read logs from 20+ nodes with 500MB log files each, or more.

## How about reading logs from kubernetes pods?

Kubernetes pods just emit logs as a stream, and by themselves they don’t have any means of *storing* the logs, unless it was specifically set up by the admin somehow, so I don’t think there can be an universal solution for nerdlog to just support any kubernetes pods. Some setup is due regardless. And as of today, one possible way to set it up is to write these logs from pods to files on some server, and then access that server with nerdlog.
