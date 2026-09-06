# Controller Profiles

This directory contains **original RC Monitor controller-profile definitions**. Profiles describe the canonical generator capabilities that the backend and future frontend may expose. They do not change RC Gateway transport behavior and do not put Modbus register maps into the Gateway core.

## Layout

```text
controllers/<manufacturer>/<model>/
  manifest.json
  telemetry.json
  alarms.json
  ui.json
```

Rapid SCADA channel bindings and templates are added later under the profile only after model/firmware documentation and validation exist. A profile is not production evidence by itself.

## Schema v1 rules

- `manifest.json` identifies one manufacturer/model and declares capabilities.
- `telemetry.json` lists only canonical `monitor.MetricKey` values.
- `alarms.json` declares normalized alarm codes/severity/messages.
- `ui.json` groups only metrics declared by the telemetry profile.
- unknown JSON fields fail closed;
- duplicate/unknown metric keys fail validation;
- component paths must be relative `.json` paths without traversal;
- `synthetic` and `draft` profiles are not HIL/production validated;
- missing telemetry is never converted to numeric zero.

The Go validator/loader lives in `internal/monitor/profile`.

## Genmon reference policy

`jgyates/genmon` is used only as a functional/domain reference. Its controller JSON files are GPLv2 material and are **not copied into this proprietary repository**. The factual manufacturer/model inventory can guide what profiles we should implement, but each RC profile must be authored from permitted manufacturer documentation, field captures/HIL, or other appropriately licensed sources.

See `REFERENCE_CATALOG.md` for the current reference inventory.
