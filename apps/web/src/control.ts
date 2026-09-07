export const CONTROL_VERSION = 1;
export const CONTROL_CHANNEL = "phonebeam-control";

export const GOLDEN_TAP =
  '{"v":1,"sid":"sid-a","seq":7,"ts":1700000000000,"cap":"input.control","type":"tap","body":{"x":0.25,"y":0.75}}';

export type ControlType =
  | "tap"
  | "swipe"
  | "back"
  | "home"
  | "ping"
  | "pong"
  | "capability_update"
  | "session_close"
  | "ack"
  | "error";

export type ControlEnvelope = {
  v: number;
  sid: string;
  seq: number;
  ts: number;
  cap: string;
  type: ControlType | string;
  body: Record<string, unknown>;
};

const TOP_KEYS = new Set(["v", "sid", "seq", "ts", "cap", "type", "body"]);
const KNOWN = new Set([
  "tap",
  "swipe",
  "back",
  "home",
  "ping",
  "pong",
  "capability_update",
  "session_close",
  "ack",
  "error",
]);

export function encodeControl(env: ControlEnvelope): string {
  return JSON.stringify({
    v: env.v,
    sid: env.sid,
    seq: env.seq,
    ts: env.ts,
    cap: env.cap,
    type: env.type,
    body: env.body,
  });
}

export function parseControl(raw: string): ControlEnvelope {
  if (!raw || raw.length > 8192) {
    throw new Error("invalid_body");
  }
  const obj = JSON.parse(raw) as Record<string, unknown>;
  const keys = Object.keys(obj);
  if (!["v", "sid", "seq", "ts", "type"].every((k) => keys.includes(k))) {
    throw new Error("invalid_body");
  }
  if (keys.some((k) => !TOP_KEYS.has(k))) {
    throw new Error("invalid_body");
  }
  if (obj.v !== CONTROL_VERSION) {
    throw new Error("unsupported_version");
  }
  const type = String(obj.type);
  if (!KNOWN.has(type)) {
    throw new Error("unknown_type");
  }
  return {
    v: Number(obj.v),
    sid: String(obj.sid),
    seq: Number(obj.seq),
    ts: Number(obj.ts),
    cap: obj.cap == null ? "" : String(obj.cap),
    type,
    body: typeof obj.body === "object" && obj.body !== null ? (obj.body as Record<string, unknown>) : {},
  };
}

export type ContentRect = { x: number; y: number; w: number; h: number };

export function containContentRect(elW: number, elH: number, videoW: number, videoH: number): ContentRect {
  if (elW <= 0 || elH <= 0 || videoW <= 0 || videoH <= 0) {
    return { x: 0, y: 0, w: 0, h: 0 };
  }
  const elAspect = elW / elH;
  const vidAspect = videoW / videoH;
  if (elAspect > vidAspect) {
    const h = elH;
    const w = h * vidAspect;
    return { x: (elW - w) / 2, y: 0, w, h };
  }
  const w = elW;
  const h = w / vidAspect;
  return { x: 0, y: (elH - h) / 2, w, h };
}

export function pointerToNormalized(
  elW: number,
  elH: number,
  videoW: number,
  videoH: number,
  localX: number,
  localY: number,
): { x: number; y: number } | null {
  const r = containContentRect(elW, elH, videoW, videoH);
  if (r.w <= 0 || r.h <= 0) {
    return null;
  }
  if (localX < r.x || localX > r.x + r.w || localY < r.y || localY > r.y + r.h) {
    return null;
  }
  return {
    x: (localX - r.x) / r.w,
    y: (localY - r.y) / r.h,
  };
}

export function controlLive(liveCaps: string[], state: string): boolean {
  return state === "CONNECTED" && liveCaps.includes("input.control");
}
