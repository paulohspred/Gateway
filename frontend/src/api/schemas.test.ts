import { describe, expect, it } from "vitest";
import { generatorCapabilitiesSchema, telemetrySchema } from "./schemas";

describe("telemetry schema", () => {
  it("aceita valores number/text/bool com quality", () => {
    const parsed = telemetrySchema.parse({ generatorId: "g1", capturedAt: "2026-09-06T12:00:00Z", communication: "online", metrics: { "generator.power_kw": { value: 0, unit: "kW", quality: "good", observedAt: "2026-09-06T12:00:00Z" }, "controller.mode": { value: "AUTO", quality: "good", observedAt: "2026-09-06T12:00:00Z" }, "breaker.gcb": { value: false, quality: "good", observedAt: "2026-09-06T12:00:00Z" } } });
    expect(parsed.metrics["generator.power_kw"]?.value).toBe(0);
    expect(parsed.metrics["breaker.gcb"]?.value).toBe(false);
  });
});


it("parses generator capabilities", () => {
  const parsed = generatorCapabilitiesSchema.parse({generatorId:"gen-1",profileId:"p1",profileStatus:"validated",telemetry:true,alarms:true,events:false,maintenance:false,remoteControl:false,metrics:[{key:"engine.rpm",displayName:"RPM",kind:"number",unit:"rpm",required:true,staleAfterSeconds:30}]});
  expect(parsed.remoteControl).toBe(false);
  expect(parsed.metrics[0]?.required).toBe(true);
});
