import { create } from "zustand";

const STORAGE_KEY = "wacalls.auth";

interface User {
  id: number;
  email: string;
  name: string;
}

interface AuthState {
  token: string | null;
  user: User | null;
  setAuth: (token: string, user: User) => void;
  logout: () => void;
  isAuthenticated: () => boolean;
}

const saved = (() => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw) as { token: string; user: User };
  } catch {}
  return null;
})();

export const useAuth = create<AuthState>((set, get) => ({
  token: saved?.token ?? null,
  user: saved?.user ?? null,

  setAuth: (token, user) => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ token, user }));
    set({ token, user });
  },

  logout: () => {
    localStorage.removeItem(STORAGE_KEY);
    set({ token: null, user: null });
  },

  isAuthenticated: () => !!get().token,
}));
