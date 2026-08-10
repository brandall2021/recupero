export type ClientStatus = "active" | "suspended" | "disabled";

export type ClientInfo = {
  id: string;
  name: string;
  slug: string;
  status: ClientStatus;
  maxSessions: number;
  usedSessions: number;
  availableSessions: number;
  createdAt: string;
  updatedAt: string;
};
