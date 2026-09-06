# Controller Reference Catalog

Status in this file means **reference only**, not supported/validated hardware.

The following controller families/models are visible in `jgyates/genmon/data/controller` and are useful for planning our own profiles. No Genmon register map or profile content is vendored here.

| Manufacturer / family | Referenced model/profile | RC status |
|---|---|---|
| Basler | DGC-2020HD | REFERENCE_ONLY |
| Briggs & Stratton | GC-1032 | REFERENCE_ONLY |
| ComAp | generic ComAp controller profile | REFERENCE_ONLY |
| Deep Sea Electronics | generic controller profile | REFERENCE_ONLY |
| Generac | Evolution Liquid Cooled | REFERENCE_ONLY |
| Kohler | APM603 | REFERENCE_ONLY |
| MEBAY | DCxx family | REFERENCE_ONLY |
| Generac / Power Zone | Power Zone 410 | REFERENCE_ONLY |
| SmartGen | HGM4000 family | REFERENCE_ONLY |

Promotion of any entry requires, at minimum:

1. exact manufacturer/model/firmware identification;
2. permitted authoritative register/channel documentation or controlled field discovery;
3. RC canonical telemetry/alarm mapping authored independently;
4. Rapid SCADA channel/template binding;
5. semantic E2E test;
6. HIL on the claimed model/firmware;
7. read-only commissioning before any command path is considered.

The first implemented profile in this repository is intentionally synthetic: `rc-simulator/reference-controller`.
