import { create } from "zustand";

const STORAGE_KEY = "wacalls.auth";

interface User {
  id: number;
  email: string;
  name: string;
  role?: string;
  clientId?: string;
}

interface AuthState {
  token: string | null;
  user: User | null;
  setAuth: (token: string, user: User) => void;
  logout: () => void;
  isAuthenticated: () => boolean;
  isPlatformAdmin: () => boolean;
}

const saved = (() => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as { token: string; user: User };
      if (!parsed.user?.role) parsed.user.role = "client_admin";
      return parsed;
    }
  } catch {}
  return null;
})();

export const useAuth = create<AuthState>((set, get) => ({
  token: saved?.token ?? null,
  user: saved?.user ?? null,

  setAuth: (token, user) => {
    if (!user.role) user.role = "client_admin";
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ token, user }));
    set({ token, user });
  },

  logout: () => {
    localStorage.removeItem(STORAGE_KEY);
    set({ token: null, user: null });
  },

  isAuthenticated: () => !!get().token,

  isPlatformAdmin: () => get().user?.role === "platform_admin",
}));
