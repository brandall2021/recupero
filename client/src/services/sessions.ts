import { apiGet, apiPost, apiPut, apiDelete } from "@/lib/api";
import { getClientId } from "@/lib/client-id";
import type { SessionInfo } from "@/types/session";

export const listSessions = () =>
  apiGet<{ sessions: SessionInfo[] }>("/api/sessions").then((r) => r.sessions ?? []);

export const listPlatformSessions = () =>
  apiGet<{ sessions: Array<SessionInfo & { state: string }> }>("/api/platform/sessions").then(
    (r) => r.sessions ?? [],
  );

export const assignSessionClient = (sid: string, clientId: string) =>
  apiPut<{ ok: boolean }>(`/api/platform/sessions/${sid}/client`, { clientId });

export const createSession = (name: string, clientId?: string) =>
  apiPost<{
    session: { id: string; name: string; status: string };
    credentials: { sessionId: string; token: string };
  }>("/api/sessions", clientId ? { name, clientId } : { name }).then((r) => r.session);

export const deleteSession = (id: string) => apiDelete(`/api/sessions/${id}`);

const postVoid = async (path: string): Promise<void> => {
  const r = await fetch(path, {
    method: "POST",
    headers: { "X-Client-Id": getClientId(), "Content-Type": "application/json" },
    body: "{}",
  });
  if (!r.ok) throw new Error(`${path} ${r.status}`);
};

export const logoutSession = (id: string) => postVoid(`/api/sessions/${id}/logout`);

export const pairSession = (id: string) => postVoid(`/api/sessions/${id}/pair`);
