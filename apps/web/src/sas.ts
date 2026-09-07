const encoder = new TextEncoder();

export const SAS_V1 = "phonebeam-sas-v1";
export const SAS_V2 = "phonebeam-sas-v2";

export function normalizeFingerprint(value: string): string {
  return value.toLowerCase().replace(/[^0-9a-f]/g, "");
}

export function validSha256Fingerprint(value: string): boolean {
  return normalizeFingerprint(value).length === 64;
}

export function decodeSasMaterial(raw: string): Uint8Array {
  const normalized = raw.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized + "=".repeat((4 - (normalized.length % 4)) % 4);
  const binary = atob(padded);
  return Uint8Array.from(binary, (ch) => ch.charCodeAt(0));
}

async function hmacDigits(key: Uint8Array, parts: string[]): Promise<string> {
  const keyCopy = new ArrayBuffer(key.byteLength);
  new Uint8Array(keyCopy).set(key);
  const cryptoKey = await crypto.subtle.importKey("raw", keyCopy, { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const data = encoder.encode(parts.join(""));
  const sig = new Uint8Array(await crypto.subtle.sign("HMAC", cryptoKey, data));
  const n = ((sig[0] << 24) | (sig[1] << 16) | (sig[2] << 8) | sig[3]) >>> 0;
  const six = n % 1_000_000;
  return `${String(Math.floor(six / 1000)).padStart(3, "0")} ${String(six % 1000).padStart(3, "0")}`;
}

function transcript(origin: string, sid: string, pid: string, operator: string, caps: string[]): string[] {
  const sorted = [...caps].sort();
  return [
    `origin=${origin}\n`,
    `sid=${sid}\n`,
    `pid=${pid}\n`,
    `operator=${operator}\n`,
    `caps=${sorted.join(",")}\n`,
  ];
}

export function pairingDisplay(
  key: Uint8Array,
  origin: string,
  sid: string,
  pid: string,
  operator: string,
  caps: string[],
): Promise<string> {
  return hmacDigits(key, [`${SAS_V1}\n`, ...transcript(origin, sid, pid, operator, caps)]);
}

export function mediaDisplay(
  key: Uint8Array,
  origin: string,
  sid: string,
  pid: string,
  operator: string,
  caps: string[],
  androidFp: string,
  browserFp: string,
): Promise<string> {
  return hmacDigits(key, [
    `${SAS_V2}\n`,
    ...transcript(origin, sid, pid, operator, caps),
    `android_fp=${normalizeFingerprint(androidFp)}\n`,
    `browser_fp=${normalizeFingerprint(browserFp)}\n`,
  ]);
}

export function extractFingerprint(sdp: string): string {
  const match = /a=fingerprint:sha-256 ([0-9A-Fa-f: ]+)/i.exec(sdp);
  return match ? normalizeFingerprint(match[1]) : "";
}

export function preserveAspect(): "contain" {
  return "contain";
}

export function hasRemoteControlUi(labels: string[]): boolean {
  return labels.some((label) => /tap|swipe|keyboard|control/i.test(label));
}
