import { canCreateSession, credentialsShouldClear, encodeQrPayload, iceAllowsPeerReady, pathFromCandidateType, remainingMs, VIEWING_STATES, viewingFailed } from "./session";
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

describe("session cleanup", () => {
  it("clears credentials on terminal states", () => {
    expect(credentialsShouldClear("CLOSED")).toBe(true);
    expect(credentialsShouldClear("EXPIRED")).toBe(true);
    expect(credentialsShouldClear("REJECTED")).toBe(true);
    expect(credentialsShouldClear("CONNECTED")).toBe(false);
    expect(credentialsShouldClear("NEGOTIATING")).toBe(false);
  });
});

describe("m2 viewing helpers", () => {
  it("treats approved projection as a viewing phase", () => {
    expect(VIEWING_STATES).toContain("NEGOTIATING");
    expect(VIEWING_STATES).toContain("CONNECTED");
    expect(VIEWING_STATES).not.toContain("QR_AVAILABLE");
  });

  it("does not mark peer ready before ICE connects", () => {
    expect(iceAllowsPeerReady("checking")).toBe(false);
    expect(iceAllowsPeerReady("connected")).toBe(true);
    expect(iceAllowsPeerReady("completed")).toBe(true);
    expect(iceAllowsPeerReady("failed")).toBe(false);
  });

  it("maps candidate types without claiming relay for host", () => {
    expect(pathFromCandidateType("relay")).toBe("relay");
    expect(pathFromCandidateType("srflx")).toBe("direct");
    expect(pathFromCandidateType("host")).toBe("direct");
  });

  it("names ICE and signaling failures", () => {
    expect(viewingFailed("FAILED_ICE")).toBe(true);
    expect(viewingFailed("FAILED_SIGNALING")).toBe(true);
    expect(viewingFailed("CONNECTED")).toBe(false);
  });
});
