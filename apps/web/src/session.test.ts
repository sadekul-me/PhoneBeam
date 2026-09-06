import { canCreateSession, encodeQrPayload, remainingMs } from "./session";
import { describe, expect, it } from "vitest";

describe("qr payload", () => {
  it("does not embed secrets", () => {
    const text = encodeQrPayload({
      v: 1,
      origin: "http://127.0.0.1:8080",
      sid: "sid",
      pid: "pid",
      exp: 1700000120,
    });
    expect(text).not.toMatch(/token|password|secret|turn/i);
    expect(JSON.parse(text).v).toBe(1);
  });
});

describe("countdown", () => {
  it("hits zero after expiry", () => {
    expect(remainingMs(new Date(0).toISOString(), 1_000)).toBe(0);
  });
});

describe("requested capabilities", () => {
  it("requires a known non-empty set", () => {
    expect(canCreateSession([])).toBe(false);
    expect(canCreateSession(["root.shell"])).toBe(false);
    expect(canCreateSession(["screen.read"])).toBe(true);
  });
});
