import { describe, expect, it } from "vitest";
import { signalingUrl } from "./signaling";

describe("signaling url", () => {
  it("does not place bearer tokens in the path", () => {
    const url = signalingUrl("sid-1");
    expect(url).toContain("/api/v1/sessions/sid-1/signal");
    expect(url).not.toMatch(/token|Bearer|password|secret/i);
  });
});
