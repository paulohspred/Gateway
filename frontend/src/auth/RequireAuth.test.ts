import { describe, expect, it } from "vitest";
import type { PublicUser } from "../api/control";
import { permissionAllowed, securitySetupPending } from "./RequireAuth";

const baseUser: PublicUser = {
  id: "u1", username: "operator", displayName: "Operator", role: "operator",
  permissions: ["fleet.read", "alarms.read"], siteScopes: ["site-a"], active: true,
  mustChangePassword: false, mfaRequired: false, mfaEnabled: false,
  createdAt: "2026-09-07T12:00:00Z", updatedAt: "2026-09-07T12:00:00Z"
};

describe("route security decisions", () => {
  it("forces setup for temporary password or pending MFA", () => {
    expect(securitySetupPending({ ...baseUser, mustChangePassword: true }, false)).toBe(true);
    expect(securitySetupPending(baseUser, true)).toBe(true);
    expect(securitySetupPending(baseUser, false)).toBe(false);
  });
  it("uses explicit permissions and fails closed", () => {
    expect(permissionAllowed(baseUser, "fleet.read")).toBe(true);
    expect(permissionAllowed(baseUser, "users.write")).toBe(false);
    expect(permissionAllowed(null, "fleet.read")).toBe(false);
  });
});
