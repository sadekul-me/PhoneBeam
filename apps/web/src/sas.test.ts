import { describe, expect, it } from "vitest";
import {
  decodeSasMaterial,
  extractFingerprint,
  hasRemoteControlUi,
  mediaDisplay,
  pairingDisplay,
  preserveAspect,
  validSha256Fingerprint,
} from "./sas";

const key = new TextEncoder().encode("phonebeam-sas-test-key-32bytes!!");

describe("sas golden vector", () => {
  it("matches coordinator media SAS 830 034", async () => {
    const got = await mediaDisplay(
      key,
      "http://127.0.0.1:8080",
      "sid-a",
      "pid-b",
      "Support-A",
      ["input.control", "screen.read"],
      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
    );
    expect(got).toBe("830 034");
  });

  it("differs from pairing SAS", async () => {
    const pairing = await pairingDisplay(
      key,
      "http://127.0.0.1:8080",
      "sid-a",
      "pid-b",
      "Support-A",
      ["input.control", "screen.read"],
    );
    const media = await mediaDisplay(
      key,
      "http://127.0.0.1:8080",
      "sid-a",
      "pid-b",
      "Support-A",
      ["input.control", "screen.read"],
      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
    );
    expect(media).not.toBe(pairing);
  });

  it("canonicalizes fingerprints", () => {
    expect(extractFingerprint("v=0\na=fingerprint:sha-256 AB:CD:EF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:01:23:45:67:89:AB:CD:EF:00:11:22:33:44\n")).toBe(
      "abcdef00112233445566778899aabbccddeeff0123456789abcdef0011223344",
    );
    expect(validSha256Fingerprint("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")).toBe(true);
  });
});

describe("operator ui constraints", () => {
  it("preserves aspect with contain", () => {
    expect(preserveAspect()).toBe("contain");
  });

  it("rejects remote-control labels", () => {
    expect(hasRemoteControlUi(["Disconnect"])).toBe(false);
    expect(hasRemoteControlUi(["Tap", "Swipe"])).toBe(true);
  });

  it("decodes raw url sas material", () => {
    const raw = btoa("hello").replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
    expect(new TextDecoder().decode(decodeSasMaterial(raw))).toBe("hello");
  });
});
