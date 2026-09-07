import { describe, expect, it } from "vitest";
import { authSessionSchema, commissioningSchema, profileStateSchema, roleSchema, settingsSchema } from "./control";

describe("control plane schemas", () => {
  it("accepts the six canonical roles and rejects unknown roles", () => {
    for (const role of ["viewer", "operator", "technician", "commissioning_engineer", "administrator", "auditor"]) expect(roleSchema.parse(role)).toBe(role);
    expect(() => roleSchema.parse("superadmin")).toThrow();
  });
  it("parses an authenticated session with site scope", () => {
    const session = authSessionSchema.parse({ user: { id:"u", username:"viewer", displayName:"Viewer", role:"viewer", permissions:["fleet.read"], siteScopes:["site-a"], active:true, mustChangePassword:false, mfaRequired:false, mfaEnabled:false, createdAt:"2026-09-07T12:00:00Z", updatedAt:"2026-09-07T12:00:00Z" }, csrfToken:"csrf", expiresAt:"2026-09-07T20:00:00Z" });
    expect(session.user.siteScopes).toEqual(["site-a"]);
  });
  it("preserves commissioning revision/lifecycle data", () => {
    const c = commissioningSchema.parse({ id:"c1", tag:"GEN-1", name:"Generator", siteId:"site-a", lifecycle:"DRAFT", stage:"IDENTITY", asset:{}, controller:{}, transport:{}, gates:{IDENTITY:{status:"PENDING",updatedAt:"2026-09-07T12:00:00Z"}}, createdBy:"u", createdAt:"2026-09-07T12:00:00Z", updatedAt:"2026-09-07T12:00:00Z", revision:2, parentCommissioningId:"c0" });
    expect(c.revision).toBe(2); expect(c.parentCommissioningId).toBe("c0");
  });
  it("validates settings and profile lifecycle enums", () => {
    expect(settingsSchema.parse({defaultLanguage:"pt-BR",defaultTimeZone:"UTC",defaultFleetView:"vertical"}).defaultFleetView).toBe("vertical");
    expect(profileStateSchema.parse({profileId:"p",status:"LAB",updatedAt:"2026-09-07T12:00:00Z",updatedBy:"u"}).status).toBe("LAB");
    expect(() => profileStateSchema.parse({profileId:"p",status:"PROD",updatedAt:"2026-09-07T12:00:00Z",updatedBy:"u"})).toThrow();
  });
});
