import { describe, expect, test } from "bun:test";
import { PhxElement, definePlugin } from "./index";

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

test("views must use own elements", () => {
  class A extends PhxElement {}
  expect(() => definePlugin({ elements: { "phx-a": A }, views: { ups: "phx-b" } })).toThrow();
  expect(definePlugin({ elements: { "phx-a": A }, views: { ups: "phx-a" } }).views?.ups).toBe("phx-a");
});
