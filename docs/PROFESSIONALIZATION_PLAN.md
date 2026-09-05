# Professionalization Plan — pre-v1 to industrial release

This document separates work that can be proven in software from decisions or tests that require repository administration, a VM or physical hardware.

## Promotion rule

No item is considered complete because code exists. Promotion requires evidence on the exact release candidate.

```text
implemented
  -> software_validated
  -> software_field_test_ready
  -> vm_accepted
  -> hil_accepted
  -> soak_accepted
  -> production_validated
```

## A. Software gates — repository/CI

Required before the first release candidate is frozen:

- bounded global and per-tunnel stream concurrency;
- unambiguous pairing model for parallel tunnels;
- real-socket parallel/concurrency tests in addition to in-memory stress;
- race detector, static analysis, vulnerability scan and CodeQL;
- minimum total coverage threshold with critical-package tests prioritized above raw percentage;
- fuzz/property tests for parsers/framing/configuration boundaries;
- strict config validation and documented schema/migration policy;
- health/readiness/operational diagnostics sufficient for field triage;
- deterministic amd64/arm64 builds, SBOM and installer attack tests;
- authenticated release/provenance design before public production distribution;
- changelog, security policy, support policy, contribution rules and threat model;
- release naming and filesystem paths free of obsolete `umbrella` branding before v1.

## B. Repository administration — owner action

These are not writable through the current GitHub integration and must be configured in repository settings before `main` is treated as protected production history:

- require pull requests for `main`;
- block direct/force pushes and branch deletion;
- require `Gateway CI` and CodeQL checks;
- require conversations to be resolved before merge;
- require review when a second qualified reviewer exists;
- enable private vulnerability reporting if available;
- verify release/tag signing policy.

`CODEOWNERS` exists for ownership routing, but a mandatory CODEOWNER approval should not be enabled while there is only one effective owner/reviewer because that can create an unusable workflow.

## C. License — owner decision after validation

No license is selected automatically. The repository owner will choose the product license after technical validation and before distribution under terms that depend on that license. Until then, do not copy third-party code merely because it is publicly visible.

## D. VM acceptance

Initial Tier-1 candidate: Ubuntu Server 24.04 LTS on amd64; arm64 becomes Tier-1 only after runtime execution tests on real/emulated arm64 are accepted.

VM acceptance must cover installation, restart, crash recovery, health/readiness, configuration rejection, upgrade, rollback, resource limits, real loopback/network sockets, firewall behavior, reboot persistence and at least a 24-hour software soak. See `VM_ACCEPTANCE.md`.

## E. Hardware-in-the-loop acceptance

Required only for capabilities claimed as physically production-validated:

- PUSR/USR/Teltonika reverse TCP and/or TCP server/client paths;
- MikroTik/VPN/4G route loss and recovery;
- RS232, RS422 and RS485 including half-duplex direction behavior;
- USB HID real enumeration, permissions, unplug/replug and ComAp InteliLite 4 AMF 9 application behavior;
- SocketCAN classic and CAN-FD with real interfaces/transceivers;
- UDP where the target deployment uses it;
- consumer restart and Gateway restart during active field traffic;
- power-cycle of modem/control equipment;
- `tc netem` or equivalent controlled latency/loss/jitter/flapping.

## F. Soak acceptance

Minimum: 24 hours. Target before broad production deployment: 7 days.

Record at fixed intervals:

- RSS/memory;
- CPU;
- file descriptors;
- goroutine count when observable through a controlled diagnostic build or equivalent evidence;
- active sessions/pairs;
- open/close/error/reconnect counters;
- bytes/datagrams/frames/reports;
- service restarts and readiness transitions.

Acceptance requires no unexplained monotonic resource growth, no payload-integrity failures and recovery within documented expectations after injected faults.

## G. Release freeze

After all required software/VM/HIL gates are green:

1. freeze the candidate commit;
2. owner chooses and adds the license;
3. rerun the full CI/security/release suite because adding release metadata changes the commit;
4. configure/verify repository protection;
5. create the signed/versioned release candidate;
6. install that exact artifact in the acceptance VM;
7. run HIL/soak against that exact version;
8. record evidence in `PROJECT_STATE.md` and the release notes;
9. only then promote status to `production validated` for the tested matrix.

## Non-goals

Professionalism does not require adding MQTT broker, historian, register database, OPC UA model server or a generic industrial command plane to the bridge core. Those are separate products/adapters unless a future requirement justifies them.
