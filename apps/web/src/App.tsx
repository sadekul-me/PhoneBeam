import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { closeSession, coordinatorBase, createSession, getSession } from "./api";
import {
  CAPABILITIES,
  credentialsShouldClear,
  formatCountdown,
  remainingMs,
  VIEWING_STATES,
  type Capability,
  type PublicSession,
} from "./session";
import { decodeSasMaterial, hasRemoteControlUi, preserveAspect } from "./sas";
import { connectSignaling, fetchIce, sendSignal } from "./signaling";
import { createViewerPeer, type ViewerHandle } from "./viewer";
import QRCode from "qrcode";

export function App() {
  const [name, setName] = useState("Support operator");
  const [requested, setRequested] = useState<Capability[]>(["screen.read"]);
  const [session, setSession] = useState<PublicSession | null>(null);
  const [token, setToken] = useState("");
  const [qrDataUrl, setQrDataUrl] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [path, setPath] = useState("connecting");
  const [mediaSas, setMediaSas] = useState("");
  const [bitrate, setBitrate] = useState(0);
  const [rtt, setRtt] = useState(0);
  const [frameSize, setFrameSize] = useState("");
  const [reconnectNonce, setReconnectNonce] = useState(0);
  const videoRef = useRef<HTMLVideoElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const viewerRef = useRef<ViewerHandle | null>(null);
  const reconnectsRef = useRef(0);
  const closingRef = useRef(false);

  function teardownMedia() {
    closingRef.current = true;
    if (wsRef.current) {
      sendSignal(wsRef.current, { v: 1, type: "hangup" });
      wsRef.current.close();
    }
    wsRef.current = null;
    viewerRef.current?.close();
    viewerRef.current = null;
    setStream(null);
    setMediaSas("");
  }

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    if (!session || !token) {
      return;
    }
    if (credentialsShouldClear(session.state)) {
      teardownMedia();
      return;
    }
    const id = window.setInterval(() => {
      void getSession(session.id, token)
        .then(setSession)
        .catch((err: unknown) => setError(err instanceof Error ? err.message : "poll_failed"));
    }, 1000);
    return () => window.clearInterval(id);
  }, [session, token]);

  useEffect(() => {
    if (!session?.qr) {
      setQrDataUrl("");
      return;
    }
    const payload = JSON.stringify(session.qr);
    void QRCode.toDataURL(payload, { margin: 1, width: 280 }).then(setQrDataUrl);
  }, [session?.qr]);

  useEffect(() => {
    const el = videoRef.current;
    if (el) {
      el.srcObject = stream;
    }
  }, [stream]);

  useEffect(() => {
    if (!session || !token || !session.sas_material) {
      return;
    }
    if (!VIEWING_STATES.includes(session.state) && session.state !== "FAILED_ICE") {
      return;
    }
    if (wsRef.current) {
      return;
    }
    const snapshot = session;
    const authToken = token;
    let cancelled = false;
    const isReconnect = reconnectsRef.current > 0;
    void (async () => {
      try {
        const ice = await fetchIce(snapshot.id, authToken);
        if (cancelled) return;
        const ws = connectSignaling(snapshot.id, authToken, {
          onMessage: (msg) => {
            if (msg.type === "hello_ok" && isReconnect) {
              sendSignal(ws, { v: 1, type: "need_offer" });
            }
            void viewerRef.current?.handleSignal(msg);
            if (msg.state) {
              setSession((cur) => (cur ? { ...cur, state: msg.state as PublicSession["state"] } : cur));
            }
          },
          onClose: () => {
            wsRef.current = null;
            viewerRef.current?.close();
            viewerRef.current = null;
            if (closingRef.current || reconnectsRef.current >= 1) {
              setPath("disconnected");
              return;
            }
            reconnectsRef.current += 1;
            setPath("reconnecting");
            setReconnectNonce((n) => n + 1);
          },
        });
        wsRef.current = ws;
        const viewer = createViewerPeer({
          ice,
          ws,
          sasKey: decodeSasMaterial(snapshot.sas_material ?? ""),
          origin: coordinatorBase(),
          sid: snapshot.id,
          pid: snapshot.pairing_id ?? snapshot.qr?.pid ?? "",
          operator: snapshot.operator_display_name,
          caps: snapshot.requested_capabilities,
          callbacks: {
            onTrack: setStream,
            onState: (state) => setSession((cur) => (cur ? { ...cur, state: state as PublicSession["state"] } : cur)),
            onPath: setPath,
            onMediaSas: setMediaSas,
            onError: (reason) => {
              setError(reason);
              if (reason === "failed_ice" && reconnectsRef.current < 1 && wsRef.current) {
                reconnectsRef.current += 1;
                sendSignal(wsRef.current, { v: 1, type: "need_offer" });
                setPath("reconnecting");
              }
            },
            onStats: (stats) => {
              if (stats.bitrateKbps) {
                setBitrate(stats.bitrateKbps);
              }
              if (stats.rttMs) {
                setRtt(stats.rttMs);
              }
              if (stats.width && stats.height) {
                setFrameSize(`${stats.width}×${stats.height}`);
              }
            },
          },
        });
        viewerRef.current = viewer;
      } catch (err) {
        setError(err instanceof Error ? err.message : "viewer_failed");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [session?.id, session?.sas_material, session?.state, token, reconnectNonce]);

  const pairingLeft = useMemo(() => {
    if (!session) {
      return 0;
    }
    return remainingMs(session.pairing_expires_at, now);
  }, [now, session]);

  const sessionLeft = useMemo(() => {
    if (!session?.session_expires_at) {
      return null;
    }
    return remainingMs(session.session_expires_at, now);
  }, [now, session]);

  const toggleCap = useCallback((cap: Capability) => {
    setRequested((current) =>
      current.includes(cap) ? current.filter((item) => item !== cap) : [...current, cap],
    );
  }, []);

  async function onCreate() {
    setError(null);
    closingRef.current = false;
    reconnectsRef.current = 0;
    try {
      const created = await createSession({ operatorDisplayName: name.trim() || "Support operator", requested });
      setSession(created.session);
      setToken(created.operatorToken);
    } catch (err) {
      setError(err instanceof Error ? err.message : "create_failed");
    }
  }

  async function onClose() {
    if (!session || !token) {
      return;
    }
    setError(null);
    teardownMedia();
    try {
      setSession(await closeSession(session.id, token));
    } catch (err) {
      setError(err instanceof Error ? err.message : "close_failed");
    }
  }

  const controlLabels = ["Disconnect"];
  const unsafeControls = hasRemoteControlUi(controlLabels);

  return (
    <main className="page">
      <header>
        <h1>PhoneBeam operator</h1>
        <p className="muted">
          M2 viewing. No remote control. Coordinator: <code>{coordinatorBase()}</code>
        </p>
      </header>

      {!session ? (
        <section className="card">
          <h2>Create session</h2>
          <label>
            Operator name
            <input value={name} onChange={(e) => setName(e.target.value)} />
          </label>
          <fieldset>
            <legend>Requested capabilities</legend>
            {CAPABILITIES.map((cap) => (
              <label key={cap} className="check">
                <input type="checkbox" checked={requested.includes(cap)} onChange={() => toggleCap(cap)} />
                {cap}
              </label>
            ))}
          </fieldset>
          <button type="button" disabled={requested.length === 0} onClick={() => void onCreate()}>
            Create session
          </button>
        </section>
      ) : (
        <section className="card">
          <p>
            <strong>State:</strong> {session.state}
          </p>
          <p>
            <strong>Connection:</strong> {path}
            {bitrate > 0 ? ` · ~${bitrate} kbps` : ""}
            {rtt > 0 ? ` · ${rtt} ms RTT` : ""}
            {frameSize ? ` · ${frameSize}` : ""}
            {session.turn_configured ? " · TURN configured" : " · TURN not configured (STUN/host only)"}
          </p>
          <p>
            Pairing expires in {formatCountdown(pairingLeft)}
            {sessionLeft !== null ? ` · session TTL ${formatCountdown(sessionLeft)}` : ""}
          </p>
          {qrDataUrl && session.state === "QR_AVAILABLE" ? (
            <img className="qr" alt="PhoneBeam pairing QR" src={qrDataUrl} />
          ) : null}
          {["PHONE_SCANNED", "APPROVAL_PENDING", "APPROVED", "CAPS_BOUND", "PROJECTION_PENDING", "PROJECTION_ACTIVE", "NEGOTIATING", "CONNECTED"].includes(session.state) && session.sas ? (
            <div className="sas">
              <p>Pairing code</p>
              <p className="sas-code">{session.sas}</p>
            </div>
          ) : null}
          {mediaSas ? (
            <div className="sas">
              <p>Verified media code</p>
              <p className="sas-code">{mediaSas}</p>
              <p className="muted">Computed locally from DTLS fingerprints. Mismatch means do not trust the video.</p>
            </div>
          ) : null}
          {session.state === "PROJECTION_DENIED" ? (
            <p className="error">Screen sharing was not started. The owner refused or lost Android capture consent.</p>
          ) : null}
          {session.state === "FAILED_ICE" ? (
            <p className="error">ICE connectivity failed. One re-handshake is attempted while capture stays live.</p>
          ) : null}
          <video
            ref={videoRef}
            className="remote"
            autoPlay
            playsInline
            style={{ objectFit: preserveAspect(), display: stream ? "block" : "none" }}
          />
          <dl>
            <dt>Requested</dt>
            <dd>{session.requested_capabilities.join(", ") || "none"}</dd>
            <dt>Effective</dt>
            <dd>{session.effective_capabilities.join(", ") || "none"}</dd>
            <dt>Capture</dt>
            <dd>
              {session.capture_scope || "unknown"}
              {session.capture_width ? ` · ${session.capture_width}×${session.capture_height}` : ""}
            </dd>
          </dl>
          {!unsafeControls ? (
            <button type="button" onClick={() => void onClose()} disabled={session.state === "CLOSED"}>
              Disconnect
            </button>
          ) : null}
        </section>
      )}
      {error ? <p className="error">{error}</p> : null}
    </main>
  );
}
