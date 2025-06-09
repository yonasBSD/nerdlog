# Troubleshooting

## Nerdlog can't connect to a host, but manually ssh-ing to that host works

TL;DR: try running nerdlog with `--set 'transport=ssh-bin'` and see if it helps.

As of now, Nerdlog uses an internal ssh implementation by default, which is pretty limited in terms of configuration, so if you have more or less advanced configuration in `~/.ssh/config`, then chances are that Nerdlog fails to parse it fully.

It does support using an external `ssh` binary instead, and it will likely help here, so as mentioned, activate it by using the  `--set 'transport=ssh-bin'` flag. The plan is to make it a default at some point, but for now it's still experimental, and thus opt-in.

## I see some logs on the histogram, but when I zoom in, Nerdlog doesn't show any logs

If you're using `journalctl`, this might be caused by [this bug in `journalctl`](https://github.com/systemd/systemd/issues/37468), where it prints inconsistent data for various overlapping time ranges.

Consider using plain log files instead; it's not the only problem with `journalctl` btw (check [FAQ](./faq.md) for details).
