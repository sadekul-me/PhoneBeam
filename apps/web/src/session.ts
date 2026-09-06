export const PROTOCOL_VERSION = 1;

export const CAPABILITIES = ["screen.read", "input.control", "audio.read"] as const;
export type Capability = (typeof CAPABILITIES)[number];

export type SessionState =
  | "SESSION_CREATED"
  | "QR_AVAILABLE"
  | "PHONE_SCANNED"
  | "APPROVAL_PENDING"
  | "APPROVED"
  | "CAPS_BOUND"
  | "REJECTED"
  | "REJECTED_CONSUMED"
  | "EXPIRED"
  | "CLOSED";

export type QrPayload = {
  v: number;
  origin: string;
  sid: string;
  pid: string;
  exp: number;
};

export type PublicSession = {
  id: string;
  state: SessionState;
  operator_display_name: string;
  requested_capabilities: string[];
  granted_capabilities: string[];
  device_capabilities: string[];
  effective_capabilities: string[];
  sas: string;
  pairing_expires_at: string;
  session_expires_at: string | null;
  qr?: QrPayload | null;
};

export function encodeQrPayload(payload: QrPayload): string {
  return JSON.stringify({
    v: payload.v,
    origin: payload.origin,
    sid: payload.sid,
    pid: payload.pid,
    exp: payload.exp,
  });
}

export function remainingMs(iso: string, now = Date.now()): number {
  const exp = Date.parse(iso);
  if (Number.isNaN(exp)) {
    return 0;
  }
  return Math.max(0, exp - now);
}

export function formatCountdown(ms: number): string {
  const total = Math.ceil(ms / 1000);
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

export function canCreateSession(caps: string[]): boolean {
  return caps.length > 0 && caps.every((name) => (CAPABILITIES as readonly string[]).includes(name));
}
