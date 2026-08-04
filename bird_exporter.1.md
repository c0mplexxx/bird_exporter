---
date: 2026-08-04
footer: bird_exporter
header: "bird_exporter's Manual"
layout: page
license: "Licensed under the MIT license"
section: 1
title: BIRD_EXPORTER
---

# NAME

bird_exporter - expose BIRD routing-daemon state as Prometheus metrics

# SYNOPSIS

**bird_exporter** [**OPTIONS**]

# DESCRIPTION

**bird_exporter** connects to one or more local BIRD Unix control sockets,
executes a fixed set of read-only **show** commands and exposes the result over
HTTP in Prometheus format.

It supports BIRD 1 IPv4/IPv6 daemons and the combined multi-channel BIRD 2/3
daemon. Metrics cover daemon status, protocol state and route counts, OSPF,
BFD, RPKI, BIRD 3 route-change limits and exporter scrape health.

# SECURITY

The standalone default **:9324** listens on all interfaces for backwards
compatibility. Bind to loopback or a protected management address. Do not
publish the metrics endpoint or a BIRD control socket to an untrusted network.

Prefer a dedicated restricted BIRD CLI socket and grant the exporter only the
socket group it requires. Restriction prevents mutating commands but does not
prevent expensive read-only commands from loading BIRD.

Built-in TLS encrypts the HTTP connection but does not authenticate clients.
Use a firewall allowlist or an authenticating/mTLS reverse proxy for remote
scraping.

# BIRD CONFIGURATION

For exact protocol uptime, configure:

```bird
timeformat protocol iso long;
```

The exporter accepts seconds, millisecond and microsecond timestamp variants.
BIRD timestamps have no timezone and are interpreted in the exporter's local
timezone.

Where supported, create a restricted socket:

```bird
cli "/run/bird/bird-exporter.ctl" {
    restrict;
};
```

# OPTIONS

**-bird.ipv4**
: Query the BIRD 1 IPv4 daemon. Default: **true**. Ignored with **-bird.v2**.

**-bird.ipv6**
: Query the BIRD 1 IPv6 daemon. Default: **true**. Ignored with **-bird.v2**.

**-bird.max-response-bytes** *bytes*
: Maximum accepted size of one BIRD socket reply. Default: **4194304**.

**-bird.socket** *path*
: BIRD or BIRD 1 IPv4 socket. Default: **/var/run/bird.ctl**.

**-bird.socket6** *path*
: BIRD 1 IPv6 socket. Default: **/var/run/bird6.ctl**. Ignored with
  **-bird.v2**.

**-bird.v2**
: Use one multi-channel socket for BIRD 2 or BIRD 3. Default: **false**.

**-format.description-labels**
: Extract optional Prometheus labels from BIRD protocol descriptions.
  Default: **false**.

**-format.description-labels-regex** *expression*
: Regular expression with exactly two capture groups for label name and value.
  Default: **(\w+)=(\w+)**.

**-format.new**
: Use the current generic metric format. Default: **true**.

**-proto.babel**
: Enable Babel metrics. Default: **true**.

**-proto.bfd**
: Enable BFD metrics. Default: **true**.

**-proto.bgp**
: Enable BGP metrics. Default: **true**.

**-proto.direct**
: Enable Direct metrics. Default: **true**.

**-proto.kernel**
: Enable Kernel metrics. Default: **true**.

**-proto.ospf**
: Enable OSPF metrics. Default: **true**.

**-proto.rpki**
: Enable RPKI metrics. Default: **true**.

**-proto.static**
: Enable Static metrics. Default: **true**.

**-tls.cert-file** *path*
: PEM certificate chain used when TLS is enabled.

**-tls.enabled**
: Serve HTTPS. Default: **false**.

**-tls.key-file** *path*
: PEM private key used when TLS is enabled.

**-version**
: Print version, revision and build date, then exit.

**-web.listen-address** *host:port*
: TCP listen address. Default: **:9324**. IPv6 literals must be bracketed.

**-web.max-concurrent-scrapes** *count*
: Maximum scrapes allowed to query BIRD concurrently. Additional requests
  receive HTTP 503. Default: **1**.

**-web.scrape-timeout** *duration*
: Whole-scrape deadline including every BIRD query. Default: **5s**.

**-web.telemetry-path** *path*
: Clean, non-root metrics path. Default: **/metrics**.

# BIRD MODES

BIRD 2/3:

```text
-bird.v2=true -bird.socket=/run/bird/bird.ctl
```

BIRD 1 IPv4 only:

```text
-bird.v2=false -bird.ipv4=true -bird.ipv6=false
-bird.socket=/run/bird/bird.ctl
```

BIRD 1 IPv6 only:

```text
-bird.v2=false -bird.ipv4=false -bird.ipv6=true
-bird.socket6=/run/bird/bird6.ctl
```

BIRD 1 dual daemon:

```text
-bird.v2=false -bird.ipv4=true -bird.ipv6=true
-bird.socket=/run/bird/bird.ctl
-bird.socket6=/run/bird/bird6.ctl
```

# HTTP CONTRACT

**GET /**
: Landing page.

**HEAD /**
: Landing-page headers.

**GET /metrics**
: Prometheus metrics, or the path configured with **-web.telemetry-path**.

Unknown paths return HTTP 404. Unsupported methods return HTTP 405. Requests
above the concurrency limit return HTTP 503 and **Retry-After: 1**. Invalid or
duplicate Prometheus metrics fail with HTTP 500 instead of returning a partial
HTTP 200 payload.

# METRICS

Prometheus provides **up** for the HTTP target. The exporter additionally
provides:

**bird_socket_query_success**
: 1 only when all BIRD queries and bounded parser operations in the scrape
  succeeded.

**bird_daemon_up**
: Parsed BIRD daemon state.

**bird_daemon_info**
: Router ID and BIRD version.

**bird_server_time_timestamp_seconds**
: BIRD-reported server time.

**bird_last_reboot_timestamp_seconds**
: Last daemon reboot.

**bird_last_reconfig_timestamp_seconds**
: Last daemon reconfiguration.

**bird_protocol_up**, **bird_protocol_uptime**
: Per-protocol/channel state and uptime.

**bird_protocol_prefix_*_count**
: Imported, exported, filtered and preferred route counts.

**bird_protocol_changes_***
: Import/export update and withdraw counters, including BIRD 3 RX-limit and
  limit values.

**bird_ospf_***, **bird_ospfv3_***
: OSPF running state, interface counts and neighbor counts.

**bird_bfd_session_***
: BFD session state, uptime, interval and timeout.

**bird_exporter_scrapes_in_flight**
: Scrapes currently querying BIRD.

**bird_exporter_scrapes_rejected_total**
: Scrapes rejected by the concurrency limit.

**bird_exporter_build_info**
: Version, source revision and Go runtime version.

# SYSTEMD

The Debian package installs **bird-exporter.service** and the mandatory
**/etc/bird_exporter/bird_exporter.env** configuration file but does not enable
or start the service.

The packaged default is:

```sh
BIRD_EXPORTER_OPTS="-web.listen-address=127.0.0.1:9324 -bird.v2=true -bird.socket=/run/bird/bird.ctl -web.scrape-timeout=5s -web.max-concurrent-scrapes=1 -bird.max-response-bytes=4194304"
```

The unit uses **DynamicUser=yes** and **SupplementaryGroups=bird**. Override
the supplementary group with a systemd drop-in when the selected socket uses a
different group.

When upgrading from **v1.6.0-hardening.1**, copy customized options from
**/etc/default/bird-exporter** to the new environment file before restarting.
The new unit does not read the old path.

# FILES

**/etc/bird_exporter/bird_exporter.env**
: Packaged command-line options. Mandatory for the packaged unit.

**/usr/lib/systemd/system/bird-exporter.service**
: Packaged hardened systemd service.

**/run/bird/bird.ctl**
: Common BIRD 2/3 control-socket location.

# LIMITATIONS

Dynamic state and filter labels can create series churn. Multiple same-family
channels with identical remaining labels can produce duplicate series and fail
the scrape. Exact uptime cannot be reconstructed from BIRD's date-only short
timestamp format.

# EXIT STATUS

The process exits non-zero for invalid configuration, listener failure, missing
TLS files or runtime server failure. Individual BIRD query failures are exposed
through metrics and logs without terminating the HTTP server.

# SEE ALSO

**bird**(8), **birdc**(8), Prometheus documentation, and the project's
**README.md** and **SECURITY.md**.

# AUTHOR

Daniel Czerwonk and contributors.
