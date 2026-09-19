import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { api } from "./api";
import type { User } from "./types";

type Auth = { user: User | null; admin: boolean; perms: string[]; loading: boolean; setUser: (u: User | null) => void; refresh: () => Promise<void> };

const Ctx = createContext<Auth>({ user: null, admin: false, perms: [], loading: true, setUser: () => {}, refresh: async () => {} });

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [admin, setAdmin] = useState(false);
  const [perms, setPerms] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const refresh = useCallback(async () => {
    try {
      const r = await api.me();
      setUser(r.user);
      setAdmin(!!r.admin);
      setPerms(r.perms ?? []);
    } catch {
      setUser(null);
      setAdmin(false);
      setPerms([]);
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  return <Ctx.Provider value={{ user, admin, perms, loading, setUser, refresh }}>{children}</Ctx.Provider>;
}

export const useAuth = () => useContext(Ctx);
