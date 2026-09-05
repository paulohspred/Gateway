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

## Phase 4 — Rapid SCADA consumer acceptance

When Rapid SCADA is part of the target deployment, install/configure the actual Rapid SCADA v6 software rather than simulating its socket behavior.

Use `docs/RAPID_SCADA_INTEGRATION.md` and one of the validated Gateway examples:

- `configs/rapid-scada.modbus-tcp.example.json`;
- `configs/rapid-scada.rtu-over-tcp.example.json`;
- `configs/rapid-scada.rs485-multidrop.example.json`.

For a co-located Linux installation, verify at minimum:

```bash
systemctl is-active scadacomm6.service
sudo RAPID_SCADA_COMM_SERVICE=scadacomm6.service \
  /opt/rc-gateway/current/scripts/rapid-scada-acceptance.sh
```

Acceptance requires the Rapid SCADA Communicator to establish a real session through the Gateway. For Modbus, verify the Rapid SCADA `TransMode` matches the payload: `TCP` for native Modbus TCP, `RTU` for RTU-over-TCP, and `ASCII` for Modbus ASCII-over-TCP.

For RS485 multidrop, use a single shared Rapid SCADA communication line/connection for the physical bus and configure unique slave IDs in Rapid SCADA. Do not create concurrent masters for the same RS485 bus.

If Rapid SCADA is hosted on another machine, set `RAPID_SCADA_REQUIRE_LOCAL_SERVICE=0`; still require an active Gateway session and record the remote Rapid SCADA version/configuration as evidence.

## Phase 5 — restart, crash and boot recovery

Verify controlled service restart, VM reboot and unexpected process termination. The service must return to readiness without manual cleanup of stale runtime sockets. A watchdog failure test should be performed in a disposable VM only, with evidence that systemd restarts the unhealthy process.

When Rapid SCADA is installed, repeat the restart sequence with `scadacomm6.service` stopped/started and prove that the consumer session is re-established without restarting the Gateway manually.

## Phase 6 — upgrade and rollback

Requires two distinct valid release artifacts.

1. Install release A and verify readiness.
2. Install release B and verify atomic `current`/`previous` state.
3. Inject a deliberately unhealthy B candidate in a disposable test to prove automatic rollback.
4. Restore B, then execute manual rollback to A.
5. Verify configuration preservation/backup retention and release metadata after each transition.
6. If Rapid SCADA is configured, verify its connection returns after both upgrade and rollback.

Never simulate rollback against a production host before it has passed on a disposable VM.

## Phase 7 — network impairment

Use `tc netem`, network namespaces or a controlled firewall to introduce delay, jitter, loss, resets and temporary route loss. Validate reconnect timing, resource bounds and absence of payload corruption.

When Rapid SCADA is in the test topology, verify the Communicator reconnects to the Gateway after temporary loss without duplicate sessions or a permanent stale connection.

## Phase 8 — soak

Minimum software VM soak: 24 hours. Target: 7 days before broad production deployment.

Collect at fixed intervals:

- `systemctl show rc-gateway.service` key resource values;
- process RSS/CPU/FDs;
- `/metrics`;
- restart count;
- active pair/session counts;
- error/reconnect counters;
- when applicable, `scadacomm6.service` state and Rapid SCADA communication errors/reconnects.

Acceptance requires no unexplained monotonic resource growth and no unexpected restart/readiness failure.

## Diagnostics

Generate a local bundle with:

```bash
sudo /opt/rc-gateway/current/scripts/collect-diagnostics.sh
```

The bundle intentionally excludes the configuration and `/sessions`, but it can still contain addresses, device identifiers and journal metadata. Sanitize it before external sharing.

## Promotion

Passing this document permits the status `vm_accepted` for the exact artifact and tested consumer combination. It does **not** validate RS232/422/485, USB HID/ComAp, CAN/CAN-FD, modem/4G/VPN or other physical combinations that were not actually connected to the VM. Those remain HIL gates.
