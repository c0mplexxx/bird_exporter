# bird-exporter Debian package

This package is named `bird-exporter`. It does not replace or declare a package
conflict with Debian's `prometheus-bird-exporter`, but both services cannot
listen on TCP port 9324 at the same time.

The package installs the binary, a hardened systemd unit, the mandatory
`/etc/default/bird-exporter` configuration and project documentation. It does
not enable or start the service.

## Before starting

1. Check the BIRD socket path, owner, group and mode:

   ```console
   stat -c '%n %U:%G %a' /run/bird/bird.ctl
   ```

2. Where supported, prefer a dedicated restricted BIRD CLI socket. A read-only
   filesystem mount does not make the BIRD command protocol read-only.

3. Review `/etc/default/bird-exporter`. The packaged BIRD 2/3 default binds to
   `127.0.0.1:9324` and limits one scrape to five seconds, one concurrent
   request and a 4 MiB BIRD reply.

4. If the socket group is not `bird`, replace
   `SupplementaryGroups=bird` with a systemd drop-in. Do not run the exporter
   as root and do not make the socket world-writable.

5. Validate and start:

   ```console
   sudo systemd-analyze verify /usr/lib/systemd/system/bird-exporter.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now bird-exporter.service
   systemctl status bird-exporter.service
   curl --fail http://127.0.0.1:9324/metrics
   ```

The environment file is intentionally mandatory. If it is missing, systemd
must fail the service rather than fall back to the standalone binary defaults
of all-interface `:9324`, BIRD 1 mode and `/var/run/bird.ctl`.

## Remote scraping

Prefer a local vmagent or Prometheus instance. Remote scraping requires a
management-only listen address and firewall allowlist or an authenticating
mTLS reverse proxy. The exporter's built-in TLS encrypts traffic but does not
authenticate clients.

See the installed `/usr/share/doc/bird-exporter/README.md` for complete BIRD 1,
BIRD 2/3, systemd, metric, sizing and troubleshooting documentation.
