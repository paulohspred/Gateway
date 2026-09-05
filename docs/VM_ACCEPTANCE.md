# VM Acceptance — RC Universal Gateway

## Objective

Validate the **installed release artifact**, not a source-tree binary, in an environment close to the first supported production host. VM acceptance sits between automated CI and physical HIL.

Initial Tier-1 candidate:

- Ubuntu Server 24.04 LTS;
- amd64 first;
- systemd;
- clean VM snapshot before install;
- network access limited to the test topology.

Arm64 remains a build target until runtime acceptance is executed on an arm64 VM/host or equivalent accepted environment.

## Evidence to record

Record VM image/version, kernel, CPU/RAM allocation, Gateway version/commit, release SHA-256, installation time, test start/end times and all injected failures. Keep the evidence with the release candidate; do not rely on memory or screenshots alone.

## Phase 1 — clean installation

1. Start from a clean snapshot.
2. Copy the exact CI-produced release artifact and checksum.
3. Run installer dry-run.
4. Install with the production installer.
5. Verify `rc-gateway.service` is enabled and active.
6. Verify it runs as user/group `rc-gateway`.
7. Verify `Type=notify` and the systemd watchdog are active.
8. Verify admin HTTP is reachable only through loopback.
9. Verify `/healthz`, `/readyz`, `/status` and `/metrics`.
10. Run `sudo /opt/rc-gateway/current/scripts/vm-acceptance.sh`.

## Phase 2 — configuration safety

Use disposable configurations to confirm:

- unknown JSON fields fail;
- trailing JSON fails;
- non-loopback data listener without allowlist fails;
- invalid TLS/mTLS combinations fail;
- duplicate sockets/devices/IDs fail;
- invalid HID selector fails;
- packet/stream mismatch without explicit framing fails;
- per-tunnel concurrency above global limits fails;
- parallel `listen↔listen` topology fails;
- valid configuration starts cleanly after each negative test.

A rejected configuration must not leave a partially started data plane.

## Phase 3 — stream concurrency and resource limits

Create a controlled loopback or network-namespace target and configure a `listen↔connect` tunnel with explicit `maxConcurrentPairs` and global `limits.maxActivePairs`.

Verify:

- multiple clients remain active simultaneously up to the per-tunnel limit;
- additional connections do not create unbounded active data-plane pairs;
- global pair count never exceeds the configured global limit across tunnels;
- payloads remain byte-identical in both directions;
- slow peers and abrupt closes recover without leaked FDs/goroutines/process growth;
- `rc_gateway_active_pairs` and per-tunnel metrics match observed sessions.

## Phase 4 — restart, crash and boot recovery

Verify controlled service restart, VM reboot and unexpected process termination. The service must return to readiness without manual cleanup of stale runtime sockets. A watchdog failure test should be performed in a disposable VM only, with evidence that systemd restarts the unhealthy process.

## Phase 5 — upgrade and rollback

Requires two distinct valid release artifacts.

1. Install release A and verify readiness.
2. Install release B and verify atomic `current`/`previous` state.
3. Inject a deliberately unhealthy B candidate in a disposable test to prove automatic rollback.
4. Restore B, then execute manual rollback to A.
5. Verify configuration preservation/backup retention and release metadata after each transition.

Never simulate rollback against a production host before it has passed on a disposable VM.

## Phase 6 — network impairment

Use `tc netem`, network namespaces or a controlled firewall to introduce delay, jitter, loss, resets and temporary route loss. Validate reconnect timing, resource bounds and absence of payload corruption.

## Phase 7 — soak

Minimum software VM soak: 24 hours. Target: 7 days before broad production deployment.

Collect at fixed intervals:

- `systemctl show rc-gateway.service` key resource values;
- process RSS/CPU/FDs;
- `/metrics`;
- restart count;
- active pair/session counts;
- error/reconnect counters.

Acceptance requires no unexplained monotonic resource growth and no unexpected restart/readiness failure.

## Diagnostics

Generate a local bundle with:

```bash
sudo /opt/rc-gateway/current/scripts/collect-diagnostics.sh
```

The bundle intentionally excludes the configuration and `/sessions`, but it can still contain addresses, device identifiers and journal metadata. Sanitize it before external sharing.

## Promotion

Passing this document permits the status `vm_accepted` for the exact artifact. It does **not** validate RS232/422/485, USB HID/ComAp, CAN/CAN-FD, modem/4G/VPN or other physical combinations that were not actually connected to the VM. Those remain HIL gates.
