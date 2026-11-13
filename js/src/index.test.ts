import { describe, expect, test } from "bun:test";
import { definePlugin } from "./index";

class Dummy {}

describe("definePlugin", () => {
  test("accepts valid plugins", () => {
    const def = definePlugin({ elements: { "phx-ups-block": Dummy as any }, blocks: [{ id: "ups", element: "phx-ups-block" }] });
    expect(def.blocks?.length).toBe(1);
  });
  test("rejects element names without phx- prefix", () => {
    expect(() => definePlugin({ elements: { "ups-block": Dummy as any } })).toThrow();
  });
  test("rejects blocks with unknown elements", () => {
    expect(() => definePlugin({ blocks: [{ id: "x", element: "phx-x" }] })).toThrow();
  });
});
