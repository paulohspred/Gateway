# Toolchain

The standalone module is `github.com/paulohspred/Gateway` and declares **Go 1.27** (`go 1.27.0`). CI uses **Go 1.27.1**.

## Runtime Go dependencies

- `go.bug.st/serial v1.8.0` — RS232/RS422/RS485 serial access;
- `golang.org/x/sys v0.43.0` — Linux syscalls used by SocketCAN/CAN-FD and related low-level paths.

TCP, TLS, Unix sockets, UDP, USB HID via `hidraw`, concurrency, configuration, admin HTTP, systemd notify/watchdog and metrics otherwise rely primarily on the standard library. USB HID deliberately avoids mandatory CGO/libusb/hidapi dependencies.

Runtime/binary redistribution notices for these dependencies and the Go runtime are retained in `THIRD_PARTY_NOTICES.md`; the release also contains a CycloneDX SBOM.

## CI quality tooling

- `actionlint` pinned to commit `914e7df21a07ef503a81201c76d2b11c789d3fca` (v1.7.12 lineage);
- Staticcheck pinned to commit `1285a6a5ec1e0ebb658f49e82b6c566a878cc3cb`;
- `govulncheck` pinned to `8fcedea455d953a0f8470e1f41420bb6f2e72665`;
- `cyclonedx-gomod` pinned to commit `07257d5b9cbd2a3d4338a880c0ca50081e1ac445` (v1.12.0 lineage).

## GitHub Actions supply chain

Third-party actions are pinned to immutable commit SHAs, with comments recording the intended major lineage:

- `actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803` (`v6` lineage);
- `actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16` (`v6` lineage);
- `actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02` (`v4` lineage);
- `github/codeql-action@cdf488f595d80d6e07e03d4674febd5ab45fa938` (`v4` lineage);
- `actions/attest-build-provenance@977bb373ede98d70efdf65b84cb5f73e068dcc2a` (`v3` lineage).

Checkout uses `persist-credentials: false`. Workflows declare explicit least-privilege permissions. Release attestation is the only workflow that requests OIDC/attestation permissions.

Dependabot is configured to propose weekly Go-module and GitHub Actions updates. Dependency update PRs must still pass the entire quality/security/release chain before adoption.

## Dependency policy

A new runtime dependency requires:

1. a concrete product requirement;
2. license compatibility review with the proprietary distribution model;
3. maintenance/supply-chain review;
4. version or immutable commit pinning where applicable;
5. `govulncheck`/CodeQL/static analysis impact review;
6. reproducible-build verification;
7. SBOM and third-party-notice updates when required.
