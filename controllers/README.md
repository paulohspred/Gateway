# Controller Profiles

This directory contains **original RC Monitor controller-profile definitions**. Profiles describe canonical generator capabilities for RC Monitor and the future frontend. They do not change RC Gateway transport behavior and do not place Modbus register maps inside the Gateway core.

## Layout

```text
controllers/<manufacturer>/<model>/
  manifest.json
  telemetry.json
  alarms.json
  ui.json
  rapid/
    channels.json
```

`rapid/channels.json` binds canonical metrics to Rapid SCADA **channel numbers**, not to physical Modbus registers. Physical register addresses/protocol details remain in the Rapid SCADA communication-line/driver template appropriate to the controller.

## Schema v1 rules

- `manifest.json` identifies one manufacturer/model and declares capabilities.
- `telemetry.json` lists only canonical `monitor.MetricKey` values.
- `alarms.json` declares normalized alarm codes/severity/messages.
- `ui.json` groups only metrics declared by the telemetry profile.
- `rapid/channels.json` maps declared metrics to Rapid SCADA channel numbers and explicit number/boolean/enum transforms.
- unknown JSON fields fail closed;
- duplicate/unknown metric keys fail validation;
- required canonical metrics must have a Rapid channel binding before that binding is accepted;
- discrete boolean/enum raw values must be explicitly mapped; unknown values fail closed;
- synthetic/draft profiles are not HIL/production validation.
- missing/undefined Rapid channel data is omitted, never converted to numeric zero.

Go validation/loading lives in `internal/monitor/profile` and the Rapid provider/binding logic in `internal/monitor/rapid`.

## Genmon reference policy

`jgyates/genmon` is used only as a functional/domain reference. Its controller JSON files are GPLv2 material and are **not copied into this proprietary repository**. The factual manufacturer/model inventory can guide which profiles should be independently authored from permitted manufacturer documentation and field/HIL evidence.

See `REFERENCE_CATALOG.md` for the current reference inventory.
