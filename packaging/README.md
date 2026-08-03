# bird-exporter Debian package

This is an upstream package named `bird-exporter`. It does not replace or
declare a package conflict with Debian's `prometheus-bird-exporter`; however,
both services cannot listen on TCP port 9324 at the same time.

The package installs, but does not enable or start, `bird-exporter.service`.
Before enabling it:

1. Check the BIRD socket path and group ownership with
   `stat -c '%n %U:%G %a' /run/bird/bird.ctl`.
2. Review `/etc/default/bird-exporter`. The packaged endpoint is intentionally
   bound to `127.0.0.1:9324`.
3. Where supported, prefer a dedicated restricted BIRD CLI socket and point
   `-bird.socket` to it. Access to the main control socket is more privileged
   than read-only metrics collection requires.
4. Run `systemctl enable --now bird-exporter.service` only after the checks.

Remote scraping requires a firewall allowlist or a TLS-authenticating reverse
proxy. Do not expose the unauthenticated metrics endpoint directly to an
untrusted network.
