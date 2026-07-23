import { create } from "zustand";

const STORAGE_KEY = "wacalls.auth";

interface AuthState {
  token: string | null;
  userId: number | null;
  username: string | null;
  setAuth: (token: string, userId: number, username: string) => void;
  logout: () => void;
  isAuthenticated: () => boolean;
}

const saved = (() => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw) as { token: string; userId: number; username: string };
  } catch {}
  return null;
})();

export const useAuth = create<AuthState>((set, get) => ({
  token: saved?.token ?? null,
  userId: saved?.userId ?? null,
  username: saved?.username ?? null,

  setAuth: (token, userId, username) => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ token, userId, username }));
    set({ token, userId, username });
  },

  logout: () => {
    localStorage.removeItem(STORAGE_KEY);
    set({ token: null, userId: null, username: null });
  },

  isAuthenticated: () => !!get().token,
}));
