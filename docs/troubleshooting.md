# Troubleshooting

## I see some logs on the histogram, but when I zoom in, Nerdlog doesn't show any logs

If you're using `journalctl`, this might be caused by [this bug in `journalctl`](https://github.com/systemd/systemd/issues/37468), where it prints inconsistent data for various overlapping time ranges.

Consider using plain log files instead; it's not the only problem with `journalctl` btw (check [FAQ](./faq.md) for details).
