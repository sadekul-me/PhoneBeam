import type { Capability, PublicSession } from "./session";

const DEFAULT_COORDINATOR = "http://127.0.0.1:8080";

export function coordinatorBase(): string {
  return (import.meta.env.VITE_COORDINATOR_ORIGIN as string | undefined) ?? DEFAULT_COORDINATOR;
}

async function parseError(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string };
    if (body.error) {
      return body.error;
    }
  } catch {
    /* ignore */
  }
  return `http_${res.status}`;
}

export async function createSession(input: {
  operatorDisplayName: string;
  requested: Capability[];
}): Promise<{ session: PublicSession; operatorToken: string }> {
  const res = await fetch(`${coordinatorBase()}/api/v1/sessions`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      operator_display_name: input.operatorDisplayName,
      requested_capabilities: input.requested,
    }),
  });
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  const body = (await res.json()) as { session: PublicSession; operator_token: string };
  return { session: body.session, operatorToken: body.operator_token };
}

export async function getSession(id: string, token: string): Promise<PublicSession> {
  const res = await fetch(`${coordinatorBase()}/api/v1/sessions/${encodeURIComponent(id)}`, {
    headers: { authorization: `Bearer ${token}` },
  });
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  const body = (await res.json()) as { session: PublicSession };
  return body.session;
}

export async function closeSession(id: string, token: string): Promise<PublicSession> {
  const res = await fetch(`${coordinatorBase()}/api/v1/sessions/${encodeURIComponent(id)}/close`, {
    method: "POST",
    headers: { authorization: `Bearer ${token}` },
  });
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  const body = (await res.json()) as { session: PublicSession };
  return body.session;
}
