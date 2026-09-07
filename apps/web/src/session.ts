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
  | "PROJECTION_PENDING"
  | "PROJECTION_ACTIVE"
  | "PROJECTION_DENIED"
  | "VIEW_NOT_REQUESTED"
  | "NEGOTIATING"
  | "CONNECTED"
  | "RECONNECTING"
  | "FAILED_ICE"
  | "FAILED_SIGNALING"
  | "PEER_AUTH_FAILED"
  | "DISCONNECTING"
  | "REJECTED"
  | "REJECTED_CONSUMED"
  | "EXPIRED"
  | "CLOSED";

export type IceServer = { urls: string[]; username?: string; credential?: string };
export type IceConfig = {
  ice_servers: IceServer[];
  ice_transport_policy: "all" | "relay";
  turn_configured: boolean;
};

export type SignalMessage = {
  v: number;
  type: string;
  token?: string;
  sdp?: { type: string; sdp: string };
  candidate?: { candidate: string; sdp_mid: string; sdp_mline_index: number };
  fingerprint?: { algorithm: string; value: string; role: string };
  state?: string;
  error?: string;
};

export type QrPayload = {
  v: number;
  origin: string;
  sid: string;
  pid: string;
  exp: number;
};

export type PublicSession = {
  id: string;
  pairing_id?: string;
  state: SessionState;
  operator_display_name: string;
  requested_capabilities: string[];
  granted_capabilities: string[];
  device_capabilities: string[];
  effective_capabilities: string[];
  sas: string;
  sas_material?: string;
  capture_scope?: string;
  capture_width?: number;
  capture_height?: number;
  connection_path?: string;
  media_trusted?: boolean;
  turn_configured?: boolean;
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

export const VIEWING_STATES: SessionState[] = [
  "CAPS_BOUND",
  "PROJECTION_PENDING",
  "PROJECTION_ACTIVE",
  "NEGOTIATING",
  "CONNECTED",
  "RECONNECTING",
];

export function credentialsShouldClear(state: SessionState): boolean {
  return state === "CLOSED" || state === "EXPIRED" || state === "REJECTED";
}

export function iceAllowsPeerReady(iceState: string): boolean {
  return iceState === "connected" || iceState === "completed";
}

export function pathFromCandidateType(type: string | undefined): "direct" | "relay" {
  return type === "relay" ? "relay" : "direct";
}

export function viewingFailed(state: SessionState): boolean {
  return state === "FAILED_ICE" || state === "FAILED_SIGNALING" || state === "PEER_AUTH_FAILED";
}

export function canCreateSession(caps: string[]): boolean {
  return caps.length > 0 && caps.every((name) => (CAPABILITIES as readonly string[]).includes(name));
}
