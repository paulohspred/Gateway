# Professionalization Plan — pre-v1 to industrial release

This document separates repository/CI work from evidence that requires repository-owner administration, a VM or physical hardware.

## Promotion rule

```text
implemented
  -> software_validated
  -> software_field_test_ready
  -> vm_accepted
  -> hil_accepted
  -> soak_accepted
  -> production_validated
```

No item is complete merely because code exists. Promotion applies only to the exact candidate that produced the evidence.

## A. Software gates — repository/CI

The pre-v1 professional baseline now includes:

- bounded global and per-tunnel stream concurrency;
- unambiguous parallel pairing policy;
- real-socket concurrency tests plus stress/churn gates;
- race detector, `go vet`, Staticcheck, CodeQL and `govulncheck`;
- workflow lint with `actionlint`;
- `go mod verify` and tidy-diff gate;
- minimum total coverage threshold;
- parser/configuration fuzz seeds and malformed-input tests;
- strict fail-closed configuration and documented schema policy;
- health/readiness/status/sessions/Prometheus diagnostics;
- systemd `Type=notify` readiness/watchdog integration;
- deterministic amd64/arm64 release builds, SHA256, SBOM and installer attack tests;
- immutable SHA pinning for third-party GitHub Actions;
- Dependabot update proposals for Go modules and Actions;
- release provenance/attestation workflow;
- changelog, security policy, support policy, threat model and contribution policy;
- canonical `rc-gateway` naming without legacy `umbrella` identity in new artifacts;
- proprietary license/notice plus third-party binary-distribution notices.

All of these remain subject to the exact-HEAD green requirement. Any code/workflow/release change invalidates the previous automated promotion until the complete gate chain is green again.

## B. Repository administration — owner action

The available GitHub integration cannot write branch-protection/ruleset or repository-visibility settings. Before `main` is treated as protected production history, the owner must apply and verify `GITHUB_PROTECTION.md`:

- require pull requests for `main`;
- require the Gateway CI and CodeQL checks;
- require resolved conversations;
- block direct/force pushes and deletion;
- limit bypass;
- enable private vulnerability reporting when available.

Because the repository is currently public, proprietary licensing prevents authorized reuse but cannot prevent source visibility or technical cloning. If the requirement is to prevent third parties from obtaining the source, the owner must make the repository private.

## C. License — selected

The selected model is **proprietary / All Rights Reserved**, not an open-source license. `LICENSE` grants no permission to use original RC Universal Gateway material without prior written authorization. `NOTICE` makes the source-visible/proprietary status explicit.

Third-party components remain under their own licenses. Required notices are retained in `THIRD_PARTY_NOTICES.md` and release SBOM metadata.

Before commercial contracting or transfer to a company, the copyright-holder identity in the proprietary notice should be reviewed by qualified legal counsel and changed to the desired legal person/entity if appropriate. That legal review does not change the technical validation gates.

## D. VM acceptance

Initial Tier-1 candidate: Ubuntu Server 24.04 LTS on amd64. Arm64 is build/reproducibility validated but should not be advertised as runtime Tier-1 until executed in a suitable arm64 acceptance environment.

The exact CI/release artifact must pass:

- clean install;
- negative/unsafe configuration rejection;
- systemd readiness and watchdog;
- restart/crash recovery;
- reboot persistence;
- loopback/network transport acceptance;
- upgrade and rollback;
- diagnostic bundle generation;
- resource observation;
- 24-hour soak, then 7-day target.

See `VM_ACCEPTANCE.md` and `scripts/vm-acceptance.sh`.

## E. Hardware-in-the-loop acceptance

Required only for capabilities claimed as physically production-validated:

- PUSR/USR/Teltonika or equivalent reverse/direct TCP paths;
- MikroTik/VPN/4G route loss and recovery;
- RS232, RS422 and RS485 including multidrop and half-duplex behavior;
- USB HID enumeration, permissions, unplug/replug and ComAp InteliLite 4 AMF 9 application behavior;
- SocketCAN classic and CAN-FD with real interfaces/transceivers;
- UDP where deployed;
- consumer/Gateway restart under active field traffic;
- power-cycle of modem/control equipment;
- controlled latency/loss/jitter/flapping.

## F. Soak acceptance

Minimum: 24 hours. Target before broad production rollout: 7 days.

Record at fixed intervals:

- RSS/memory and CPU;
- file descriptors;
- active sessions/pairs;
- open/close/error/reconnect counters;
- bytes/datagrams/frames/reports;
- service restarts/readiness transitions;
- any payload-integrity failure.

Acceptance requires no unexplained monotonic resource growth and recovery within documented expectations after injected faults.

## G. Release freeze

1. finish repository-only changes;
2. obtain all automated gates green on the exact candidate;
3. apply/verify repository-owner protection settings;
4. install that exact artifact in the acceptance VM;
5. complete VM soak/upgrade/rollback tests;
6. run HIL for each claimed physical path;
7. record evidence in `PROJECT_STATE.md` and release notes;
8. create/sign the versioned release from the accepted commit;
9. promote only the tested matrix to `production validated`.

## Non-goals

Professionalism does not require adding an MQTT broker, historian, register database, OPC UA model server or generic industrial command plane to the bridge core. Those remain separate products/adapters unless a future requirement justifies them.
