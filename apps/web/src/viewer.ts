import { extractFingerprint, mediaDisplay } from "./sas";
import { sendSignal } from "./signaling";
import { iceAllowsPeerReady, pathFromCandidateType, type IceConfig, type SignalMessage } from "./session";
import { CONTROL_CHANNEL, parseControl, type ControlEnvelope } from "./control";

export type ViewerCallbacks = {
  onTrack: (stream: MediaStream) => void;
  onState: (state: string) => void;
  onPath: (path: "connecting" | "direct" | "relay" | "reconnecting" | "disconnected") => void;
  onMediaSas: (code: string) => void;
  onError: (reason: string) => void;
  onStats?: (stats: { bitrateKbps?: number; rttMs?: number; width?: number; height?: number }) => void;
  onControl?: (msg: ControlEnvelope) => void;
};

export type ViewerHandle = {
  handleSignal: (msg: SignalMessage) => Promise<void>;
  close: () => void;
  sendControl: (env: ControlEnvelope) => boolean;
};

export function createViewerPeer(input: {
  ice: IceConfig;
  ws: WebSocket;
  sasKey: Uint8Array;
  origin: string;
  sid: string;
  pid: string;
  operator: string;
  caps: string[];
  callbacks: ViewerCallbacks;
}): ViewerHandle {
  const pc = new RTCPeerConnection({
    iceServers: input.ice.ice_servers.map((s) => ({
      urls: s.urls,
      username: s.username,
      credential: s.credential,
    })),
    iceTransportPolicy: input.ice.ice_transport_policy === "relay" ? "relay" : "all",
  });
  let localFp = "";
  let remoteFp = "";
  let readySent = false;
  let iceUp = false;
  const bytesSample = { bytes: 0, at: 0 };
  let control: RTCDataChannel | null = null;

  pc.addEventListener("datachannel", (ev) => {
    if (ev.channel.label !== CONTROL_CHANNEL) {
      ev.channel.close();
      return;
    }
    control = ev.channel;
    control.addEventListener("message", (msg) => {
      try {
        input.callbacks.onControl?.(parseControl(String(msg.data)));
      } catch {
        /* ignore malformed phone messages */
      }
    });
  });

  pc.addEventListener("track", (ev) => {
    const stream = ev.streams[0] ?? new MediaStream([ev.track]);
    input.callbacks.onTrack(stream);
  });
  pc.addEventListener("icecandidate", (ev) => {
    if (!ev.candidate) {
      sendSignal(input.ws, { v: 1, type: "ice_complete" });
      return;
    }
    sendSignal(input.ws, {
      v: 1,
      type: "ice_candidate",
      candidate: {
        candidate: ev.candidate.candidate,
        sdp_mid: ev.candidate.sdpMid ?? "0",
        sdp_mline_index: ev.candidate.sdpMLineIndex ?? 0,
      },
    });
  });
  pc.addEventListener("iceconnectionstatechange", () => {
    if (iceAllowsPeerReady(pc.iceConnectionState)) {
      iceUp = true;
      void maybeReady();
      void detectPath(pc, input.callbacks.onPath);
      void reportInboundStats(pc, bytesSample, input.callbacks.onStats);
    }
    if (pc.iceConnectionState === "disconnected") {
      iceUp = false;
      input.callbacks.onPath("reconnecting");
    }
    if (pc.iceConnectionState === "failed") {
      iceUp = false;
      sendSignal(input.ws, { v: 1, type: "failed_ice" });
      input.callbacks.onError("failed_ice");
      input.callbacks.onPath("disconnected");
    }
  });

  async function maybeReady() {
    if (readySent || !iceUp || !localFp || !remoteFp) {
      return;
    }
    const code = await mediaDisplay(
      input.sasKey,
      input.origin,
      input.sid,
      input.pid,
      input.operator,
      input.caps,
      remoteFp,
      localFp,
    );
    input.callbacks.onMediaSas(code);
    readySent = true;
    sendSignal(input.ws, { v: 1, type: "peer_ready" });
  }

  return {
    async handleSignal(msg: SignalMessage) {
      if (msg.type === "sdp_offer" && msg.sdp) {
        readySent = false;
        iceUp = false;
        remoteFp = extractFingerprint(msg.sdp.sdp);
        await pc.setRemoteDescription({ type: "offer", sdp: msg.sdp.sdp });
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);
        localFp = extractFingerprint(answer.sdp ?? "");
        sendSignal(input.ws, { v: 1, type: "sdp_answer", sdp: { type: "answer", sdp: answer.sdp ?? "" } });
        sendSignal(input.ws, {
          v: 1,
          type: "peer_fingerprint",
          fingerprint: { algorithm: "sha-256", value: localFp, role: "operator" },
        });
        await maybeReady();
      }
      if (msg.type === "ice_candidate" && msg.candidate) {
        await pc.addIceCandidate({
          candidate: msg.candidate.candidate,
          sdpMid: msg.candidate.sdp_mid,
          sdpMLineIndex: msg.candidate.sdp_mline_index,
        });
      }
      if (msg.type === "state" && msg.state) {
        input.callbacks.onState(msg.state);
      }
      if (msg.type === "hangup") {
        pc.close();
        input.callbacks.onPath("disconnected");
      }
    },
    close() {
      control?.close();
      control = null;
      pc.close();
    },
    sendControl(env: ControlEnvelope) {
      if (!control || control.readyState !== "open") {
        return false;
      }
      control.send(JSON.stringify({
        v: env.v,
        sid: env.sid,
        seq: env.seq,
        ts: env.ts,
        cap: env.cap,
        type: env.type,
        body: env.body,
      }));
      return true;
    },
  };
}

async function detectPath(pc: RTCPeerConnection, onPath: ViewerCallbacks["onPath"]): Promise<void> {
  const stats = await pc.getStats();
  let path: "direct" | "relay" = "direct";
  stats.forEach((report) => {
    if (report.type === "candidate-pair" && "state" in report && report.state === "succeeded") {
      const remoteId = "remoteCandidateId" in report ? String(report.remoteCandidateId) : "";
      const remote = stats.get(remoteId);
      const typ = remote && "candidateType" in remote ? String(remote.candidateType) : undefined;
      path = pathFromCandidateType(typ);
    }
  });
  onPath(path);
}

async function reportInboundStats(
  pc: RTCPeerConnection,
  sample: { bytes: number; at: number },
  onStats: ViewerCallbacks["onStats"],
): Promise<void> {
  const stats = await pc.getStats();
  let bitrateKbps: number | undefined;
  let rttMs: number | undefined;
  let width: number | undefined;
  let height: number | undefined;
  stats.forEach((report) => {
    if (report.type === "inbound-rtp" && "bytesReceived" in report) {
      const bytes = Number(report.bytesReceived);
      const now = "timestamp" in report ? Number(report.timestamp) : Date.now();
      if (sample.at > 0 && now > sample.at) {
        bitrateKbps = Math.max(0, Math.round(((bytes - sample.bytes) * 8) / (now - sample.at)));
      }
      sample.bytes = bytes;
      sample.at = now;
      if ("frameWidth" in report) {
        width = Number(report.frameWidth);
      }
      if ("frameHeight" in report) {
        height = Number(report.frameHeight);
      }
    }
    if (report.type === "candidate-pair" && "state" in report && report.state === "succeeded") {
      if ("currentRoundTripTime" in report) {
        rttMs = Math.round(Number(report.currentRoundTripTime) * 1000);
      }
    }
  });
  onStats?.({ bitrateKbps, rttMs, width, height });
}
