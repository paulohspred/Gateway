# Support Policy

## Product scope

RC Universal Gateway transports industrial communication between field and consumer endpoints. Supported transport does not mean the Gateway understands the device register map or converts one industrial application protocol into another.

## Triage information

For a reproducible support case, provide non-secret versions of:

- Gateway version and commit (`rc-gateway --version`);
- Linux distribution/kernel and architecture;
- sanitized configuration;
- topology and transport type;
- relevant `/status`, `/sessions` and `/metrics` output;
- journal excerpts around the failure;
- exact failure time and recovery behavior;
- hardware/model/firmware only when the issue is hardware-specific.

Never attach credentials, private keys, VPN secrets or sensitive production payloads to a public issue.

## Validation levels

- **implemented** — code exists;
- **software validated** — corresponding automated tests passed;
- **software field-test-ready** — all automated quality/security/release gates passed on the exact commit;
- **production validated** — documented VM/HIL, failure/recovery and soak criteria passed for the real deployment combination.

## Platform support

Until the VM acceptance program is complete, Linux amd64/arm64 builds are release targets, not a claim that every Linux distribution is supported. Ubuntu Server 24.04 is the initial Tier-1 VM/HIL candidate.

## Protocol/application support

Rapid SCADA, FUXA, manufacturer software or another consumer remains responsible for interpreting application protocols unless a separate, explicitly documented adapter is introduced. USB HID support, for example, preserves reports; it does not automatically convert ComAp Direct to Modbus.
