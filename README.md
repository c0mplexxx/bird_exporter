# bird_exporter

[![Go Report Card](https://goreportcard.com/badge/github.com/czerwonk/bird_exporter)](https://goreportcard.com/report/github.com/czerwonk/bird_exporter)

`bird_exporter` reads operational state from the BIRD routing daemon's Unix
control socket and exposes it in Prometheus text format. It supports BIRD 1
dual-daemon installations and BIRD 2/3 multi-channel installations.

The exporter is intended to run on the same host as BIRD. It only issues a
fixed set of `show ...` commands, but the process still needs read/write access
to the selected BIRD control socket.

## Features

- daemon status, version, router ID and BIRD timestamps;
- BGP, OSPF, Kernel, Static, Direct, Babel and RPKI protocol state;
- imported, exported, filtered and preferred route counts;
- protocol uptime and BIRD 2/3 route-change counters;
- BIRD 3 RX-limit and limit counters;
- OSPF interface, neighbor and adjacent-neighbor counts;
- BFD session state, uptime, interval and timeout;
- optional labels extracted from protocol descriptions;
- legacy and current metric naming formats;
- bounded BIRD replies, a whole-scrape timeout and overload shedding;
- optional server-side TLS;
- a hardened Debian/systemd package.

## Security model

The standalone binary keeps the historical `:9324` all-interface default for
compatibility. The Debian package overrides this with `127.0.0.1:9324`. Do not
expose the metrics endpoint directly to the Internet or another untrusted
network. Prometheus also recommends protecting exporter endpoints from public
access because they expose operational data and can be used to generate load:
[Prometheus security model](https://prometheus.io/docs/operating/security/).

Prefer a dedicated restricted BIRD CLI socket where the installed BIRD version
supports it. A restricted socket only limits commands; BIRD documents that an
expensive read-only request can still overload the daemon, so the socket must
not be public:
[BIRD remote-control documentation](https://bird.nic.cz/doc/bird-2.19.0.html#ss4.1).

The exporter applies three independent resource bounds by default:

```text
-web.scrape-timeout=5s
-web.max-concurrent-scrapes=1
-bird.max-response-bytes=4194304
```

Keep description-derived labels disabled unless their keys and cardinality are
controlled. The exporter rejects reserved, duplicate, oversized or excessive
description labels, but valid high-cardinality values can still create many
Prometheus series.

## BIRD compatibility

### BIRD 2 and BIRD 3

Use `-bird.v2=true`. IPv4 and IPv6 channels are read from one daemon and one
socket. In this mode `-bird.ipv4`, `-bird.ipv6` and `-bird.socket6` are
ignored.

BIRD 3 adds RX-limit and limit columns to route-change statistics. The exporter
detects and exports those additional columns automatically.

### BIRD 1

BIRD 1 normally uses separate `bird` and `bird6` daemons. Leave
`-bird.v2=false` and enable the daemon(s) that exist:

- IPv4 only: `-bird.ipv4=true -bird.ipv6=false`;
- IPv6 only: `-bird.ipv4=false -bird.ipv6=true`;
- dual daemon: `-bird.ipv4=true -bird.ipv6=true`.

Set `-bird.socket` and `-bird.socket6` to the actual paths.

## Required BIRD preparation

### Use stable protocol timestamps

BIRD 2/3 defaults to `iso short ms` for protocol timestamps. The exporter
accepts near-time values with seconds, milliseconds or microseconds, but the
default format becomes date-only for older sessions and no longer contains
enough information for an exact uptime.

Use this global BIRD setting:

```bird
timeformat protocol iso long;
```

BIRD timestamps do not include a timezone. The exporter interprets them in its
local timezone. A native systemd service naturally uses the host timezone; a
container must set `TZ` to the BIRD host timezone.

### Prefer a restricted socket

On BIRD versions that support additional CLI sockets:

```bird
cli "/run/bird/bird-exporter.ctl" {
    restrict;
};
```

Reload BIRD using the site's normal safe procedure, then inspect the socket:

```console
stat -c '%n %U:%G %a' /run/bird/bird-exporter.ctl
birdc -s /run/bird/bird-exporter.ctl 'show status'
birdc -s /run/bird/bird-exporter.ctl 'show protocols all'
```

The exporter needs permission to connect and write commands to the socket.
Grant access through the narrow socket group; never solve this with mode `0666`
or by running the exporter as root.

## Debian/systemd quick start

The repository's GoReleaser configuration produces a package named
`bird-exporter`. It does not conflict at the package-manager level with
`prometheus-bird-exporter`, but two exporters cannot listen on TCP port 9324 at
the same time.

### 1. Install a verified package

Download the package and `checksums.txt` from the same release, then verify the
artifact before installation:

```console
sha256sum -c checksums.txt --ignore-missing
sudo dpkg -i bird-exporter_<version>_linux_amd64.deb
```

The package installs the following files:

```text
/usr/bin/bird_exporter
/usr/lib/systemd/system/bird-exporter.service
/etc/default/bird-exporter
/usr/share/doc/bird-exporter/
```

It intentionally does not enable or start the service.

### 2. Check the socket and edit the configuration

```console
stat -c '%n %U:%G %a' /run/bird/bird.ctl
sudoedit /etc/default/bird-exporter
```

The packaged BIRD 2/3 configuration is:

```sh
BIRD_EXPORTER_OPTS="-web.listen-address=127.0.0.1:9324 -bird.v2=true -bird.socket=/run/bird/bird.ctl -web.scrape-timeout=5s -web.max-concurrent-scrapes=1 -bird.max-response-bytes=4194304"
```

Point `-bird.socket` to the dedicated restricted socket if one was configured.
The environment file is mandatory: deleting it makes the unit fail instead of
silently returning to the binary's all-interface and BIRD 1 defaults.

### 3. Validate and start

```console
sudo systemd-analyze verify /usr/lib/systemd/system/bird-exporter.service
sudo systemctl daemon-reload
sudo systemctl enable --now bird-exporter.service
systemctl status bird-exporter.service
journalctl -u bird-exporter.service -n 100 --no-pager
curl --fail --silent --show-error http://127.0.0.1:9324/metrics
```

### BIRD 1 examples

IPv4 only:

```sh
BIRD_EXPORTER_OPTS="-web.listen-address=127.0.0.1:9324 -bird.v2=false -bird.ipv4=true -bird.ipv6=false -bird.socket=/run/bird/bird.ctl -web.scrape-timeout=5s -web.max-concurrent-scrapes=1 -bird.max-response-bytes=4194304"
```

IPv6 only:

```sh
BIRD_EXPORTER_OPTS="-web.listen-address=127.0.0.1:9324 -bird.v2=false -bird.ipv4=false -bird.ipv6=true -bird.socket6=/run/bird/bird6.ctl -web.scrape-timeout=5s -web.max-concurrent-scrapes=1 -bird.max-response-bytes=4194304"
```

Dual daemon:

```sh
BIRD_EXPORTER_OPTS="-web.listen-address=127.0.0.1:9324 -bird.v2=false -bird.ipv4=true -bird.ipv6=true -bird.socket=/run/bird/bird.ctl -bird.socket6=/run/bird/bird6.ctl -web.scrape-timeout=5s -web.max-concurrent-scrapes=1 -bird.max-response-bytes=4194304"
```

Restart the service after changing `/etc/default/bird-exporter`:

```console
sudo systemctl restart bird-exporter.service
journalctl -u bird-exporter.service -n 100 --no-pager
```

### Socket group override

The packaged unit uses `DynamicUser=yes` and `SupplementaryGroups=bird`. If the
socket belongs to another group, use a systemd drop-in:

```console
sudo systemctl edit bird-exporter.service
```

```ini
[Service]
SupplementaryGroups=
SupplementaryGroups=bird-exporter-socket
```

The empty assignment resets the packaged list. Include every group the process
needs on the replacement line. Verify the effective unit with:

```console
systemctl cat bird-exporter.service
systemctl show bird-exporter.service -p DynamicUser -p SupplementaryGroups
```

## Prometheus or vmagent

The packaged loopback listener expects the scraper to run on the same host.
Keep the scraper timeout slightly larger than the exporter's internal timeout:

```yaml
scrape_configs:
  - job_name: bird
    scrape_interval: 15s
    scrape_timeout: 7s
    static_configs:
      - targets:
          - 127.0.0.1:9324
```

The scrape interval should be greater than the scrape timeout. Do not compensate
for slow BIRD responses by enabling many concurrent scrapes; measure the BIRD
reply, tune the response limit and investigate the slow command first.

## HTTP contract

- `GET /metrics` performs a scrape;
- `GET /` and `HEAD /` return the landing page;
- other paths return `404`;
- unsupported methods return `405`;
- a scrape above `-web.max-concurrent-scrapes` returns `503` with
  `Retry-After: 1`;
- invalid or internally inconsistent Prometheus metrics fail the scrape with
  HTTP `500` instead of returning a partial HTTP `200` payload.

A BIRD transport or parser query failure is represented by
`bird_socket_query_success 0`. Other metrics that were collected safely may
still be present in that scrape, so alert on this metric separately.

## Metric semantics

Prometheus adds its own `up` metric for the HTTP target. It must not be confused
with exporter metrics:

| Signal | Meaning |
|---|---|
| `up` | Prometheus or vmagent completed the HTTP scrape successfully |
| `bird_socket_query_success` | Every BIRD socket query and bounded parser operation in this scrape succeeded |
| `bird_daemon_up` | The parsed BIRD daemon state is up |
| `bird_protocol_up` | A specific BIRD protocol/channel is up |

The current metric format uses these families:

| Family | Purpose |
|---|---|
| `bird_daemon_info{router_id,version}` | Static daemon identity |
| `bird_server_time_timestamp_seconds` | BIRD-reported server time |
| `bird_last_reboot_timestamp_seconds` | Last BIRD reboot |
| `bird_last_reconfig_timestamp_seconds` | Last BIRD reconfiguration |
| `bird_protocol_up{...,state}` | Protocol state |
| `bird_protocol_uptime` | Protocol uptime in whole seconds |
| `bird_protocol_prefix_import_count` | Imported routes |
| `bird_protocol_prefix_export_count` | Exported routes |
| `bird_protocol_prefix_filter_count` | Filtered routes |
| `bird_protocol_prefix_preferred_count` | Preferred routes |
| `bird_protocol_changes_*` | Import/export update and withdraw counters, including BIRD 3 limits |
| `bird_ospf_*` / `bird_ospfv3_*` | OSPF state, interfaces and neighbors |
| `bird_bfd_session_*` | BFD session state and timing |
| `bird_exporter_scrapes_in_flight` | Scrapes currently querying BIRD |
| `bird_exporter_scrapes_rejected_total` | Scrapes rejected by the concurrency limit |
| `bird_exporter_build_info{version,revision,go_version}` | Binary provenance |

The common protocol labels are `name`, `proto`, `ip_version`,
`import_filter` and `export_filter`. `bird_protocol_up` also has `state`.

Set `-format.new=false` only for compatibility with dashboards that still use
the pre-1.3 protocol-specific names such as `bgp4_session_*` and
`bgp6_session_*`. New deployments should use the default format.

## Command-line options

| Option | Default | Description |
|---|---:|---|
| `-bird.ipv4` | `true` | Query the BIRD 1 IPv4 daemon; ignored with `-bird.v2` |
| `-bird.ipv6` | `true` | Query the BIRD 1 IPv6 daemon; ignored with `-bird.v2` |
| `-bird.max-response-bytes` | `4194304` | Maximum accepted size of one BIRD reply |
| `-bird.socket` | `/var/run/bird.ctl` | BIRD/BIRD 1 IPv4 socket |
| `-bird.socket6` | `/var/run/bird6.ctl` | BIRD 1 IPv6 socket |
| `-bird.v2` | `false` | Use one multi-channel socket for BIRD 2 or BIRD 3 |
| `-format.description-labels` | `false` | Extract optional labels from protocol descriptions |
| `-format.description-labels-regex` | `(\w+)=(\w+)` | Regex with exactly two capture groups: label name and value |
| `-format.new` | `true` | Use the current generic metric format |
| `-proto.babel` | `true` | Enable Babel metrics |
| `-proto.bfd` | `true` | Enable BFD metrics |
| `-proto.bgp` | `true` | Enable BGP metrics |
| `-proto.direct` | `true` | Enable Direct metrics |
| `-proto.kernel` | `true` | Enable Kernel metrics |
| `-proto.ospf` | `true` | Enable OSPF metrics |
| `-proto.rpki` | `true` | Enable RPKI metrics |
| `-proto.static` | `true` | Enable Static metrics |
| `-tls.cert-file` | empty | PEM certificate chain |
| `-tls.enabled` | `false` | Serve HTTPS |
| `-tls.key-file` | empty | PEM private key |
| `-version` | `false` | Print build information and exit |
| `-web.listen-address` | `:9324` | TCP listen address; must be valid `host:port` |
| `-web.max-concurrent-scrapes` | `1` | Scrapes allowed to query BIRD concurrently |
| `-web.scrape-timeout` | `5s` | Whole-scrape deadline, including all BIRD queries |
| `-web.telemetry-path` | `/metrics` | Clean, non-root metrics path |

Boolean flags accept explicit values, for example `-bird.v2=false`.

## Response-limit sizing

Measure the largest real reply using the same socket and group permissions as
the exporter:

```console
birdc -s /run/bird/bird.ctl 'show protocols all' | wc -c
```

Set `-bird.max-response-bytes` above the observed production maximum with
headroom for peer growth and format changes. Do not set an unlimited value. A
too-small value produces `bird_socket_query_success 0` and a
`response too large` error in the service log.

## Remote scraping and TLS

The preferred pattern is a local vmagent/Prometheus scraper and a loopback
exporter. If remote scraping is required:

1. bind only to a management address, not `0.0.0.0`;
2. allow only the scraper addresses in the host/network firewall;
3. use an authenticating reverse proxy or service mesh for mTLS;
4. keep the BIRD Unix socket local.

Built-in TLS is enabled with:

```text
-tls.enabled=true
-tls.cert-file=/etc/bird-exporter/tls.crt
-tls.key-file=/etc/bird-exporter/tls.key
```

It encrypts the connection but does not request or verify client certificates.
It is not an authentication boundary.

With the packaged `DynamicUser` unit, store the private key outside `/home` and
grant it through a dedicated group and systemd drop-in. Keep the key mode at
`0640` or stricter and include both the BIRD socket group and TLS group in
`SupplementaryGroups`.

## Container and Kubernetes notes

The supplied container runs as UID/GID 1000. Add only the host socket group,
for example with Docker `--group-add`; do not run the container as root.

The Helm chart and example DaemonSet mount the host BIRD socket directory.
Their numeric `fsGroup` must match the group that can access the socket on the
node. A read-only volume mount does not make the BIRD command protocol
read-only. Apply a NetworkPolicy or equivalent cluster-network restriction to
the metrics Service.

## Troubleshooting

### Service does not start

```console
systemctl status bird-exporter.service
journalctl -u bird-exporter.service -b --no-pager
systemctl cat bird-exporter.service
```

- `Failed to load environment files`: restore and review
  `/etc/default/bird-exporter`.
- `invalid web listen address`: use `host:port`; bracket IPv6, for example
  `[::1]:9324`.
- `address already in use`: check `ss -ltnp 'sport = :9324'` and stop the
  conflicting exporter or choose another port.

### Permission denied on the BIRD socket

```console
namei -l /run/bird/bird.ctl
stat -c '%n %U:%G %a' /run/bird/bird.ctl
systemctl show bird-exporter.service -p DynamicUser -p SupplementaryGroups
```

Fix the socket group or systemd drop-in. Do not widen the socket to all users.

### HTTP 503

The concurrency limit is protecting BIRD. Check scrape overlap, interval,
timeout and `bird_exporter_scrapes_rejected_total`. Increasing concurrency
also increases concurrent control-socket work and is not the first fix.

### HTTP 500

The exporter rejected invalid or duplicate Prometheus metrics rather than
serving a partial payload. Check the journal for unsupported OSPF address
families, duplicate series from multiple same-family channels, or inconsistent
description-label keys.

### Uptime is zero

- configure `timeformat protocol iso long;`;
- verify the exporter timezone matches the BIRD host;
- confirm the output with `birdc ... 'show protocols all'`;
- date-only BIRD timestamps cannot provide exact uptime.

### No IPv6 or unexpected socket errors

Check the selected mode:

- BIRD 2/3 requires `-bird.v2=true` and one `-bird.socket`;
- BIRD 1 IPv6 requires `-bird.v2=false -bird.ipv6=true` and
  `-bird.socket6`;
- `-bird.socket6` is ignored in BIRD 2/3 mode.

## Known limitations

- The `state`, `import_filter` and `export_filter` labels can create series
  churn when BIRD state or configuration changes. Removing them is a breaking
  metric change tracked in
  [issue #63](https://github.com/czerwonk/bird_exporter/issues/63).
- Multiple channels with the same protocol name, address family and remaining
  label values can produce duplicate series. The exporter fails that scrape
  instead of returning partial data. A lossless fix requires a future channel
  or table identity label and a migration plan.
- Different description-label key sets within one metric family can make the
  scrape invalid. Keep a stable key schema across all protocols.
- Exact uptime cannot be reconstructed from BIRD's date-only short format.
- The exporter does not provide client authentication and does not make a BIRD
  control socket safe for public access.

## Build and test

Go 1.26.5 or newer is required.

```console
go mod verify
go mod tidy -diff
go test -count=1 ./...
go test -race ./...
go vet ./...
go build -trimpath -o bird_exporter .
```

A release build must work from a clean checkout with `GOWORK=off` and without a
local `replace` directive.

## Dashboards

Example Grafana dashboards for BGP, OSPF/BFD and route-server use cases are in
the [grafana directory](grafana/).

## Reporting security issues

Follow [SECURITY.md](SECURITY.md). Do not publish exploit details in a public
issue.

## License

Copyright Daniel Czerwonk and contributors. Licensed under the
[MIT License](LICENSE).
