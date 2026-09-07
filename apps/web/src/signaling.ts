import { coordinatorBase } from "./api";
import type { IceConfig, SignalMessage } from "./session";

export type SignalHandlers = {
  onMessage: (msg: SignalMessage) => void;
  onClose: () => void;
};

export function signalingUrl(sessionId: string): string {
  const http = coordinatorBase();
  const ws = http.replace(/^https:/, "wss:").replace(/^http:/, "ws:");
  return `${ws}/api/v1/sessions/${encodeURIComponent(sessionId)}/signal`;
}

export function connectSignaling(sessionId: string, token: string, handlers: SignalHandlers): WebSocket {
  const ws = new WebSocket(signalingUrl(sessionId));
  ws.addEventListener("open", () => {
    ws.send(JSON.stringify({ v: 1, type: "hello", token }));
  });
  ws.addEventListener("message", (ev) => {
    handlers.onMessage(JSON.parse(String(ev.data)) as SignalMessage);
  });
  ws.addEventListener("close", () => handlers.onClose());
  return ws;
}

export async function fetchIce(sessionId: string, token: string): Promise<IceConfig> {
  const res = await fetch(`${coordinatorBase()}/api/v1/sessions/${encodeURIComponent(sessionId)}/ice`, {
    headers: { authorization: `Bearer ${token}` },
  });
  if (!res.ok) {
    throw new Error("ice_config_failed");
  }
  return (await res.json()) as IceConfig;
}

export function sendSignal(ws: WebSocket, msg: SignalMessage): void {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(msg));
  }
}
