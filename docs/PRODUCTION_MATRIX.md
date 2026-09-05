# Production Readiness Matrix — RC Universal Gateway

> Canonical state: [`PROJECT_STATE.md`](./PROJECT_STATE.md). Procedures: [`RUNBOOK.md`](./RUNBOOK.md). Compatibility: [`COMPATIBILITY_MATRIX.md`](./COMPATIBILITY_MATRIX.md).

The quality target is transport integrity, bounded resource behavior, fail-closed configuration and recoverability. A larger protocol catalog is not a substitute for those properties.

## Core and transport plane

| Capability | Software evidence | Physical gate |
|---|---|---|
| raw duplex stream bridge | unit/integration tests | target topology |
| TCP listen/connect | real sockets + integration tests | modem/device/VPN |
| bounded parallel listen→connect pairs | real socket tests + global/per-tunnel limits | load profile |
| byte-for-byte bidirectional transport | regression tests | target consumer/device |
| pair/write/drain timeouts | tests | network impairment |
| reconnect/churn | 1,000-cycle gate | modem/VPN flapping |
| scale/concurrency | 1,000-pair gate | capacity acceptance |
| FD/goroutine leak behavior | automated gate | long soak |
| TLS 1.3/mTLS | tests + fail-closed config | real PKI/topology |
| Unix stream | tested | local integration |
| Unix `SOCK_SEQPACKET` | message-boundary tests | local integration |
| packet↔stream `length32be` | bidirectional framing tests | compatible adapter |
| RS232/RS422/RS485 | provider/config tests | physical interfaces/bus |
| USB HID `/dev/hidrawN` | transport/safety tests | real HID/application protocol |
| USB HID VID/PID/serial | discovery/re-enumeration tests | real IDs/device behavior |
| UDP | bounded-session tests | real UDP target |
| SocketCAN/CAN-FD | provider/frame tests | interface/transceiver |
| metrics/sessions/status | tests | operator acceptance |

## Security and availability

| Gate | Current design |
|---|---|
| admin exposure | loopback-only |
| admin methods | GET-only operational endpoints |
| HTTP limits | header/read/write/idle timeouts |
| public TCP/UDP listener | CIDR allowlist required |
| TLS options with TLS off | rejected |
| mTLS without CA | rejected |
| CAN transmit | disabled by default |
| USB HID write | disabled by default |
| provider socket/path collisions | rejected |
| HID symlink/non-character device | rejected |
| HID ambiguous selector | rejected |
| HID path/identity mismatch | rejected |
| provider socket type mismatch | rejected |
| packet↔stream without framing | rejected |
| unsafe archive entries | rejected/tested |
| active stream pairs | globally and per-tunnel bounded |
| UDP sessions | bounded |
| readiness | barrier after local component initialization |
| fatal component error | coordinated cancellation/shutdown |
| systemd liveness | `Type=notify` + watchdog |

## Configuration

| Gate | State |
|---|---|
| strict JSON / unknown fields | enforced |
| trailing JSON document | rejected |
| single-snapshot `LoadStrict` | implemented |
| IDs/binds/sockets/devices unique | validated |
| physical serial/USB conflicts | validated |
| provider socket network type | validated |
| packet framing compatibility | validated |
| global/per-tunnel resource limits | validated/defaulted |
| `--check-config` without opening transports | implemented/tested |
| all `configs/*.json` examples | CI gate |
| schema policy | `CONFIGURATION_COMPATIBILITY.md` |

## Automated candidate gates

The exact candidate must pass, on the same HEAD:

1. canonical-state consistency;
2. `actionlint` for all workflows;
3. formatting;
4. `go mod verify` and tidy-diff;
5. `go vet`;
6. Staticcheck;
7. shuffled unit/integration tests with coverage;
8. minimum total coverage threshold;
9. race detector;
10. build and config-example validation;
11. 1,000 concurrent-pair stress and real-socket concurrency tests;
12. 1,000 TCP churn cycles/leak gate;
13. impairment + mini-soak;
14. `govulncheck`;
15. CodeQL Go;
16. shell syntax and malicious installer tests;
17. reproducible amd64/arm64 builds;
18. SHA256 + CycloneDX SBOM;
19. installer dry-run against the real archive;
20. release-content checks, including license and third-party notices;
21. artifact publication.

Third-party GitHub Actions are pinned to immutable commit SHAs. Dependabot proposes dependency/action updates, but no update is trusted without the same gates.

## Release and provenance

The canonical release contains only `rc-gateway` identity and paths. It includes:

- deterministic amd64/arm64 binaries with embedded version/commit/build date;
- `MANIFEST` and `VERSION`;
- proprietary `LICENSE` and `NOTICE`;
- `THIRD_PARTY_NOTICES.md` and CycloneDX SBOM;
- SHA256 checksum;
- hardened systemd unit;
- validated config examples;
- install, rollback, diagnostics, USB probe, VM acceptance and soak scripts;
- operator documentation.

The installer accepts only a single-root archive containing regular files/directories, validates product/license identity, validates the candidate config with the candidate binary, retains bounded config backups, switches releases atomically and health-gates activation/rollback.

Version-tag release builds have a dedicated provenance-attestation workflow using GitHub OIDC and immutable action pinning.

## Licensing state

Original RC Universal Gateway code is proprietary / All Rights Reserved. This is intentionally **not** an open-source license. Third-party components retain their own license rights and notices.

Repository public visibility does not grant reuse permission. If source access itself must be restricted, repository visibility must be changed to private by the owner; licensing alone cannot prevent a public repository from being read or cloned.

## Promotion levels

- **implemented** — code exists;
- **software validated** — corresponding automated tests pass;
- **software field-test-ready** — every automated candidate/security/release gate is green on the exact HEAD;
- **VM accepted** — the exact artifact passes clean-VM install/upgrade/rollback/reboot/watchdog/soak acceptance;
- **HIL accepted** — the exact artifact passes the claimed physical hardware/network/consumer matrix;
- **production validated** — VM/HIL/soak evidence for the claimed matrix is recorded.

## Remaining non-CI gates

Repository owner must still configure `main` protection/rulesets because the available GitHub integration cannot write administrative settings. See `GITHUB_PROTECTION.md`.

Physical/VM gates remain:

- clean Ubuntu Server 24.04 VM acceptance;
- 24-hour soak, with 7-day target;
- PUSR/USR/Teltonika or equivalent reverse/direct TCP hardware;
- MikroTik/VPN/4G loss/recovery;
- RS232, RS422, RS485 and multidrop bus behavior;
- USB HID and ComAp InteliLite 4 AMF 9 application behavior;
- UDP where deployed;
- CAN/CAN-FD hardware;
- power-cycle/reconnect and consumer/Gateway restart scenarios.

CI cannot substitute for those physical claims.
