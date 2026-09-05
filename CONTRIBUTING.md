# Contributing to RC Universal Gateway

## Engineering contract

The core remains bridge-first:

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

A transport/provider must preserve the semantics of its medium. Protocol interpretation, register maps, polling, historian behavior and generic industrial command planes do not belong in the core.

## Proprietary contribution policy

RC Universal Gateway is proprietary software. The repository being publicly visible does not grant a license to use its code; see `LICENSE` and `NOTICE`.

External code contributions are **not accepted by default**. Do not submit source-code changes, copied snippets, generated implementations, or derivative works unless the repository owner has explicitly invited the contribution and the necessary written intellectual-property terms have been agreed first. This avoids ambiguous ownership and incompatible inbound licensing.

Bug reports, interoperability observations, hardware test results, and security reports are welcome when they do not disclose third-party confidential material or production secrets. Security reports must follow `SECURITY.md`.

## Change process for authorized contributors

1. Work on a branch; do not push production changes directly to `main`.
2. Keep changes focused and include tests for behavior and failure paths.
3. Run `bash scripts/ci.sh` before requesting review.
4. Update `docs/PROJECT_STATE.md` whenever runtime, security, configuration, release or transport contracts change.
5. Keep configuration fail-closed and backward-compatible where practical. If compatibility must break, document migration before merge.
6. Never weaken write/transmit safeguards merely to make a test or device work.
7. Do not add dependencies without reviewing license, maintenance, supply-chain impact and reproducible-build implications.
8. Preserve proprietary and third-party notices in all permitted copies and release artifacts.

## Required quality

Changes affecting the data plane should cover byte/frame/report integrity, shutdown, timeout, reconnect and resource cleanup where applicable. Concurrency changes require race-detector coverage and bounded resource behavior. Parser/configuration changes should include malformed-input tests or fuzzing where useful.

## Pull requests

An authorized PR should explain the problem, design decision, security impact, compatibility impact, validation performed and any remaining physical/HIL gate. A green CI run proves software validation only; it does not replace field validation.

## External code

Ideas and public specifications may inform an independent implementation. Do not copy code with incompatible licensing into this repository. Record material third-party dependencies and their licenses in the normal Go module/SBOM chain and in `THIRD_PARTY_NOTICES.md` when binary-distribution notice obligations apply.
