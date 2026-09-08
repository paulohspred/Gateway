# Configuration Compatibility Policy

This document defines how RC Universal Gateway configuration compatibility is handled before and after the first production release.

## Canonical schema

`schema: 3` is the current canonical pre-v1 schema. Configuration is parsed strictly: unknown fields, trailing JSON documents, conflicting resources, unsafe listener exposure, invalid TLS combinations, incompatible provider socket types and invalid packet-framing combinations are rejected.

A configuration that passes `rc-gateway --check-config` is syntactically and structurally valid for that exact binary version. It is not proof that the attached field hardware or application protocol is reachable.

## Pre-v1 rule

Before v1.0.0 there is no promise that historical experimental schemas remain accepted indefinitely. When a pre-v1 change is intentionally incompatible, the change must be documented in `CHANGELOG.md`, `PROJECT_STATE.md` and the release notes before promotion.

The first production release will freeze the supported baseline. No production installation should depend on an undocumented pre-v1 schema.

## Post-v1 rule

After v1.0.0:

- a patch release must not introduce an intentional incompatible configuration change;
- a minor release should remain backward-compatible whenever possible;
- an incompatible schema change requires an explicit new schema number and documented migration path;
- configuration must never be silently reinterpreted when that could change exposure, transport semantics, write/transmit behavior or physical-device selection;
- automatic migration, if introduced, must write a new candidate file and never overwrite the only known-good configuration before validation;
- `--check-config` remains the mandatory pre-install/pre-restart gate.

## Migration safety contract

A future migration tool must be deterministic and fail-closed. It must:

1. read the source configuration once;
2. identify the source schema explicitly;
3. reject unknown or ambiguous input rather than guessing;
4. produce a separate output file;
5. validate the resulting configuration with the target schema;
6. preserve security-sensitive intent, including allowlists, TLS, write/transmit opt-ins, physical-device selectors, resource limits and packet framing;
7. report every semantic change made by the migration;
8. leave the original file untouched on failure.

## Current installation behavior

The release installer validates the candidate configuration with the candidate binary before changing the active release. It retains bounded configuration backups and performs health-gated rollback if the new release does not become ready.

The canonical installation paths are:

```text
/opt/rc-gateway
/etc/rc-gateway.json
/etc/systemd/system/rc-gateway.service
```

Legacy `rc-gateway-umbrella` installations are deliberately not migrated implicitly. They require an explicit migration procedure so that a new release cannot accidentally take ownership of an old installation layout.
