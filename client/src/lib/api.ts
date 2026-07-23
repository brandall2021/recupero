import { getClientId } from "./client-id";
import { useAuth } from "@/stores/auth";

const baseHeaders = (): HeadersInit => {
  const h: Record<string, string> = {
    "X-Client-Id": getClientId(),
    "Content-Type": "application/json",
  };
  const token = useAuth.getState().token;
  if (token) h["Authorization"] = `Bearer ${token}`;
  return h;
};

export const apiGet = async <T>(path: string): Promise<T> => {
  const r = await fetch(path, { headers: baseHeaders() });
  if (!r.ok) throw new Error(`${path} ${r.status}`);
  return r.json() as Promise<T>;
};

export const apiPost = async <T>(path: string, body: unknown): Promise<T> => {
  const r = await fetch(path, { method: "POST", headers: baseHeaders(), body: JSON.stringify(body) });
  if (!r.ok) {
    const text = await r.text().catch(() => "");
    throw new Error(`${path} ${r.status} ${text}`);
  }
  return r.json() as Promise<T>;
};

export const apiDelete = async (path: string): Promise<void> => {
  const r = await fetch(path, { method: "DELETE", headers: baseHeaders() });
  if (!r.ok) throw new Error(`${path} ${r.status}`);
};
