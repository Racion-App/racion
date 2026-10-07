import { useEffect, useMemo, useRef, useState, type ReactElement } from "react";
import { MessageCircle, Send } from "lucide-react";
import { renderSVG } from "uqr";
import { api } from "../lib/api";
import { track } from "../lib/analytics";
import { inMiniApp, type MiniPlatform } from "../lib/miniapp";
import { useT } from "../i18n";

// Вход через бота Telegram или MAX на обычном сайте. Кнопка заводит запрос входа на сервере и
// показывает ссылку на бота (на компьютере ещё и QR-код для телефона). Человек жмёт в боте «Войти»,
// страница раз в две секунды спрашивает сервер и, когда вход подтверждён, уходит дальше. Ждать
// может только этот браузер: секрет ожидания лежит в его cookie. Внутри мессенджера кнопок нет —
// там вход в одно касание.

const NAME: Record<MiniPlatform, string> = { telegram: "Telegram", max: "MAX" };
const ICON: Record<MiniPlatform, ReactElement> = {
  telegram: <Send size={20} aria-hidden />,
  max: <MessageCircle size={20} aria-hidden />,
};
const WAIT_MS = 10 * 60 * 1000; // столько живёт запрос на сервере

// useMessengerBots — у каких мессенджеров подключён бот; внутри мини-приложения — ни у каких.
export function useMessengerBots(): MiniPlatform[] {
  const [bots, setBots] = useState<MiniPlatform[]>([]);
  useEffect(() => {
    if (inMiniApp()) return;
    let alive = true;
    api.meta().then((m) => {
      if (alive) setBots((["max", "telegram"] as const).filter((p) => m.bots?.[p]));
    }).catch(() => {});
    return () => { alive = false; };
  }, []);
  return bots;
}

export function MessengerLoginButton({ platform, onStart }: { platform: MiniPlatform; onStart: (p: MiniPlatform) => void }) {
  const { t } = useT();
  return (
    <button type="button" className="btn btn-soft oauth__btn" onClick={() => onStart(platform)} aria-label={t("auth.via.one", { name: NAME[platform] })}>
      {ICON[platform]}
      <span>{NAME[platform]}</span>
    </button>
  );
}

// MessengerWait — ожидание подтверждения в боте: ссылка, QR на компьютере, опрос сервера.
export function MessengerWait({ platform, plan, next, onClose }: { platform: MiniPlatform; plan?: string; next: string; onClose: () => void }) {
  const { t } = useT();
  const [url, setUrl] = useState("");
  const [state, setState] = useState<"start" | "wait" | "expired" | "error">("start");
  const busy = useRef(false);
  const desktop = useMemo(() => typeof window !== "undefined" && window.matchMedia("(hover: hover) and (pointer: fine) and (min-width: 640px)").matches, []);

  // запрос входа: новая попытка — новый запрос (кнопка «Попробовать снова» меняет ключ компонента)
  useEffect(() => {
    let alive = true;
    fetch(`/api/auth/messenger/${platform}/start`, {
      method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ plan: plan ?? "" }),
    })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
      .then((d: { url: string }) => { if (alive) { setUrl(d.url); setState("wait"); } })
      .catch(() => { if (alive) setState("error"); });
    return () => { alive = false; };
  }, [platform, plan]);

  useEffect(() => {
    if (state !== "wait") return;
    let stop = false;
    const until = Date.now() + WAIT_MS;
    const poll = async () => {
      if (stop || busy.current) return;
      busy.current = true;
      try {
        const r = await fetch(`/api/auth/messenger/${platform}/poll`, { method: "POST", credentials: "same-origin" });
        if (stop) return;
        if (r.status === 200) {
          stop = true;
          track("auth_bot_" + platform);
          window.location.assign(next); // полная загрузка: сессия уже в cookie, кабинет и шапка подтянут её сами
          return;
        }
        if (r.status === 410 || Date.now() > until) {
          stop = true;
          setState("expired");
        }
      } catch {
        // сеть пропала на секунду — спросим в следующий раз
      } finally {
        busy.current = false;
      }
    };
    const id = window.setInterval(poll, 2000);
    // из мессенджера человек возвращается во вкладку: спросить сразу, не дожидаясь таймера
    const onVisible = () => { if (document.visibilityState === "visible") void poll(); };
    document.addEventListener("visibilitychange", onVisible);
    return () => { stop = true; window.clearInterval(id); document.removeEventListener("visibilitychange", onVisible); };
  }, [state, platform, next]);

  const qr = useMemo(() => (desktop && url ? renderSVG(url, { border: 1 }) : ""), [desktop, url]);
  const name = NAME[platform];

  return (
    <div className="mlogin" role="status" aria-live="polite">
      {state === "expired" || state === "error" ? (
        <p className="mlogin__text">{t(state === "expired" ? "auth.messenger.expired" : "auth.oauth.error")}</p>
      ) : (
        <>
          <p className="mlogin__text">{t("auth.messenger.wait", { name })}</p>
          {url && (
            <a className="btn btn-primary mlogin__open" href={url} target="_blank" rel="noopener">
              {ICON[platform]} {t("auth.messenger.open", { name })}
            </a>
          )}
          {qr && (
            <figure className="mlogin__qr">
              <span dangerouslySetInnerHTML={{ __html: qr }} />
              <figcaption>{t("auth.messenger.qr")}</figcaption>
            </figure>
          )}
        </>
      )}
      <button type="button" className="btn btn-ghost mlogin__cancel" onClick={onClose}>{t("auth.messenger.cancel")}</button>
    </div>
  );
}
