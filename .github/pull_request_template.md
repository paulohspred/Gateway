## Problem

Describe the operational/engineering problem this change solves.

## Design

Describe the chosen design and why it preserves the bridge-first contract.

## Security / safety impact

- [ ] No new external listener or privilege
- [ ] Write/transmit behavior remains fail-closed or explicitly authorized
- [ ] Secrets are not logged or committed
- [ ] Resource usage remains bounded

Explain any checked item that is not applicable or any new trust boundary.

## Compatibility

Describe configuration, API, transport, packaging or migration impact.

## Validation

- [ ] `gofmt`
- [ ] `go vet`
- [ ] unit/integration tests
- [ ] race detector where applicable
- [ ] configuration examples
- [ ] stress/failure tests where applicable
- [ ] `docs/PROJECT_STATE.md` updated for runtime/security/release changes

## Physical / VM gates

List any HIL, VM, modem, serial, HID, CAN, VPN, power-cycle or soak validation still required. Do not mark software CI as equivalent to production validation.
