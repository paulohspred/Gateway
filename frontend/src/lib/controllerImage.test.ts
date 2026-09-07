import { describe, expect, it } from "vitest";
import { controllerImageSrc } from "./controllerImage";

describe("controllerImageSrc", () => {
  it("maps only reviewed model assets", () => {
    expect(controllerImageSrc("DSE8610")).toBe("/controllers/dse8610.png");
    expect(controllerImageSrc("ComAp InteliLite 9")).toBe("/controllers/intelilite9.png");
    expect(controllerImageSrc("unknown controller")).toBe("/controllers/generic.png");
  });
});
