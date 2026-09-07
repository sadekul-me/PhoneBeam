import { describe, expect, it } from "vitest";
import {
  GOLDEN_TAP,
  containContentRect,
  controlLive,
  encodeControl,
  parseControl,
  pointerToNormalized,
} from "./control";
import { hasRemoteControlUi } from "./sas";

describe("control protocol", () => {
  it("parses the Kotlin golden tap vector", () => {
    const env = parseControl(GOLDEN_TAP);
    expect(env.v).toBe(1);
    expect(env.sid).toBe("sid-a");
    expect(env.seq).toBe(7);
    expect(env.type).toBe("tap");
    expect(env.body.x).toBe(0.25);
    expect(env.body.y).toBe(0.75);
    expect(encodeControl(env)).toBe(GOLDEN_TAP);
  });

  it("rejects unknown version and types", () => {
    expect(() => parseControl('{"v":2,"sid":"s","seq":1,"ts":1,"cap":"input.control","type":"tap","body":{}}')).toThrow(
      "unsupported_version",
    );
    expect(() => parseControl('{"v":1,"sid":"s","seq":1,"ts":1,"cap":"input.control","type":"keyboard","body":{}}')).toThrow(
      "unknown_type",
    );
  });
});

describe("letterbox coordinates", () => {
  it("maps clicks inside contained video to normalized frame space", () => {
    const rect = containContentRect(1000, 500, 1000, 2000);
    expect(rect.w).toBeCloseTo(250);
    expect(rect.x).toBeCloseTo(375);
    const hit = pointerToNormalized(1000, 500, 1000, 2000, 500, 250);
    expect(hit).not.toBeNull();
    expect(hit?.x).toBeCloseTo(0.5);
    expect(hit?.y).toBeCloseTo(0.5);
  });

  it("rejects letterbox clicks", () => {
    expect(pointerToNormalized(1000, 500, 1000, 2000, 10, 250)).toBeNull();
  });
});

describe("control ux gating", () => {
  it("hides control unless connected and live", () => {
    expect(controlLive(["screen.read"], "CONNECTED")).toBe(false);
    expect(controlLive(["screen.read", "input.control"], "NEGOTIATING")).toBe(false);
    expect(controlLive(["screen.read", "input.control"], "CONNECTED")).toBe(true);
  });

  it("does not treat keyboard as an M3 control", () => {
    expect(hasRemoteControlUi(["Back", "Home"])).toBe(false);
    expect(hasRemoteControlUi(["Keyboard"])).toBe(true);
  });
});
