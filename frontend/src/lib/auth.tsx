import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { api } from "./api";
import { inTelegram, tgInitData } from "./telegram";
import type { User } from "./types";

// tg — итог входа в мини-приложении Telegram: status от сервера, open — куда вести по параметру запуска.
type TgState = { status: "login" | "created" | "linked" | "other" | "guest"; open: string };

type Auth = {
  user: User | null;
  admin: boolean;
  perms: string[];
  links: string[]; // к каким сервисам привязан вход (telegram, vk…)
  tg: TgState | null;
  loading: boolean;
  setUser: (u: User | null) => void;
  refresh: () => Promise<void>;
  tgLogin: (plan?: string) => Promise<void>;
};

const Ctx = createContext<Auth>({ user: null, admin: false, perms: [], links: [], tg: null, loading: true, setUser: () => {}, refresh: async () => {}, tgLogin: async () => {} });

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [admin, setAdmin] = useState(false);
  const [perms, setPerms] = useState<string[]>([]);
  const [links, setLinks] = useState<string[]>([]);
  const [tg, setTg] = useState<TgState | null>(null);
  const [loading, setLoading] = useState(true);
  const refresh = useCallback(async () => {
    try {
      const r = await api.me();
      setUser(r.user);
      setAdmin(!!r.admin);
      setPerms(r.perms ?? []);
      setLinks(r.links ?? []);
    } catch {
      setUser(null);
      setAdmin(false);
      setPerms([]);
      setLinks([]);
    }
  }, []);
  // В Telegram при каждом открытии сверяемся с сервером: привязанный аккаунт входит сам, вошедший на
  // сайте привязывает Telegram. Пока не ответил — loading, иначе кабинет успел бы отправить на вход.
  useEffect(() => {
    void (async () => {
      await refresh();
      if (inTelegram()) {
        try {
          const r = await api.webAppAuth("telegram", tgInitData());
          setTg({ status: r.status, open: r.open ?? "" });
          if (r.status !== "guest" && r.status !== "other") await refresh();
        } catch {
          // подпись устарела или нет сети: остаёмся гостем, сайт работает и так
        }
      }
      setLoading(false);
    })();
  }, [refresh]);
  const tgLogin = useCallback(
    async (plan?: string) => {
      const r = await api.webAppAuth("telegram", tgInitData(), true, plan);
      setTg({ status: r.status, open: "" });
      await refresh();
    },
    [refresh],
  );
  return <Ctx.Provider value={{ user, admin, perms, links, tg, loading, setUser, refresh, tgLogin }}>{children}</Ctx.Provider>;
}

export const useAuth = () => useContext(Ctx);
