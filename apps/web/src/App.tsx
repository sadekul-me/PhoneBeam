import { useCallback, useEffect, useMemo, useState } from "react";
import { closeSession, coordinatorBase, createSession, getSession } from "./api";
import { CAPABILITIES, formatCountdown, remainingMs, type Capability, type PublicSession } from "./session";
import QRCode from "qrcode";

export function App() {
  const [name, setName] = useState("Support operator");
  const [requested, setRequested] = useState<Capability[]>(["screen.read"]);
  const [session, setSession] = useState<PublicSession | null>(null);
  const [token, setToken] = useState("");
  const [qrDataUrl, setQrDataUrl] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    if (!session || !token) {
      return;
    }
    if (["CLOSED", "EXPIRED", "REJECTED"].includes(session.state)) {
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
    try {
      setSession(await closeSession(session.id, token));
    } catch (err) {
      setError(err instanceof Error ? err.message : "close_failed");
    }
  }

  return (
    <main className="page">
      <header>
        <h1>PhoneBeam operator</h1>
        <p className="muted">
          M1 pairing only. No remote screen. Coordinator: <code>{coordinatorBase()}</code>
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
            <strong>Session:</strong> <code>{session.id}</code>
          </p>
          <p>
            Pairing expires in {formatCountdown(pairingLeft)}
            {sessionLeft !== null ? ` · session TTL ${formatCountdown(sessionLeft)}` : ""}
          </p>
          {qrDataUrl && session.state === "QR_AVAILABLE" ? (
            <img className="qr" alt="PhoneBeam pairing QR" src={qrDataUrl} />
          ) : null}
          {["PHONE_SCANNED", "APPROVAL_PENDING", "APPROVED", "CAPS_BOUND"].includes(session.state) && session.sas ? (
            <div className="sas">
              <p>PhoneBeam verification code</p>
              <p className="sas-code">{session.sas}</p>
              <p className="muted">Read this aloud and confirm it matches the phone.</p>
            </div>
          ) : null}
          <dl>
            <dt>Requested</dt>
            <dd>{session.requested_capabilities.join(", ") || "none"}</dd>
            <dt>Granted</dt>
            <dd>{session.granted_capabilities.join(", ") || "none"}</dd>
            <dt>Device</dt>
            <dd>{session.device_capabilities.join(", ") || "none"}</dd>
            <dt>Effective</dt>
            <dd>{session.effective_capabilities.join(", ") || "none"}</dd>
          </dl>
          <button type="button" onClick={() => void onClose()} disabled={session.state === "CLOSED"}>
            Close session
          </button>
        </section>
      )}
      {error ? <p className="error">{error}</p> : null}
    </main>
  );
}
