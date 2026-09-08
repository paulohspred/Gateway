# Security Policy

## Status

RC Universal Gateway is currently pre-v1 and undergoing software, VM and hardware-in-the-loop validation. Security fixes may change behavior when required to preserve fail-closed operation.

## Reporting a vulnerability

Do **not** publish exploit details, credentials, device identifiers or field topology in a public issue.

Use GitHub Security Advisories / private vulnerability reporting for this repository when that option is available. If the private reporting control is unavailable, open a minimal public issue stating only that you need a private security contact; do not include technical vulnerability details in that issue.

A useful private report includes:

- affected commit/version and platform;
- attack preconditions and trust boundary;
- reproducible steps or proof of concept;
- expected versus observed behavior;
- impact on confidentiality, integrity or availability;
- suggested mitigation, if known.

## Security boundaries

The Gateway is bridge-first. A working transport path does not authorize industrial writes and does not imply semantic validation of the application protocol. Administrative HTTP is loopback-only by design. Public data listeners require explicit allowlists, and dangerous transmit/write capabilities remain opt-in where the transport permits a safe read-only default.

## Secrets

Do not commit private keys, certificates with private material, passwords, APN credentials, VPN secrets, production IP inventories or captured industrial payloads containing sensitive data. Test fixtures must use synthetic/non-secret material.

## Supported security baseline

Before a release is promoted, its exact commit must pass the repository security/release gates, including vulnerability scanning, race tests, installer archive security tests, reproducible builds and SBOM generation. Production validation additionally requires the documented VM/HIL and soak gates.
