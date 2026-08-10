import { apiGet, apiPost, apiDelete, apiPut, apiPatch } from "@/lib/api";
import type { ClientInfo, ClientStatus } from "@/types/client";

export const listClients = () =>
  apiGet<{ clients: ClientInfo[] }>("/api/platform/clients").then((r) => r.clients ?? []);

export const createClient = (body: {
  name: string;
  slug?: string;
  maxSessions?: number;
  admin: { name?: string; email: string; password: string };
}) =>
  apiPost<{ client: ClientInfo; admin: { email: string; name: string; role: string } }>(
    "/api/platform/clients",
    body,
  );

export const updateClient = (id: string, body: { name?: string; maxSessions?: number }) =>
  apiPut<{ client: ClientInfo }>(`/api/platform/clients/${id}`, body).then((r) => r.client);

export const setClientStatus = (id: string, status: ClientStatus) =>
  apiPatch<{ client: ClientInfo }>(`/api/platform/clients/${id}/status`, { status }).then((r) => r.client);

export const setClientLimits = (id: string, maxSessions: number) =>
  apiPatch<{ client: ClientInfo }>(`/api/platform/clients/${id}/limits`, { maxSessions }).then((r) => r.client);

export const deleteClient = (id: string) => apiDelete(`/api/platform/clients/${id}`);
