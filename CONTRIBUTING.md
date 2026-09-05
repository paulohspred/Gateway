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

## Change process

1. Work on a branch; do not push production changes directly to `main`.
2. Keep changes focused and include tests for behavior and failure paths.
3. Run `bash scripts/ci.sh` before requesting review.
4. Update `docs/PROJECT_STATE.md` whenever runtime, security, configuration, release or transport contracts change.
5. Keep configuration fail-closed and backward-compatible where practical. If compatibility must break, document migration before merge.
6. Never weaken write/transmit safeguards merely to make a test or device work.
7. Do not add dependencies without reviewing license, maintenance, supply-chain impact and reproducible-build implications.

## Required quality

Changes affecting the data plane should cover byte/frame/report integrity, shutdown, timeout, reconnect and resource cleanup where applicable. Concurrency changes require race-detector coverage and bounded resource behavior. Parser/configuration changes should include malformed-input tests or fuzzing where useful.

## Pull requests

A PR should explain the problem, design decision, security impact, compatibility impact, validation performed and any remaining physical/HIL gate. A green CI run proves software validation only; it does not replace field validation.

## External code

Ideas and public specifications may inform an independent implementation. Do not copy code with incompatible licensing into this repository. Record material third-party dependencies and their licenses in the normal Go module/SBOM chain.
