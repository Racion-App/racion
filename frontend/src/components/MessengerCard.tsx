import { useEffect, useState } from "react";
import { Check, Send } from "lucide-react";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { openInTelegram } from "../lib/telegram";
import { useConfirm } from "./Confirm";
import { useT } from "../i18n";

// MessengerCard — привязка Telegram в кабинете. Привязанный аккаунт входит в мини-приложении без
// пароля, а бот присылает списки недель этого аккаунта. Привязка идёт через бота: кабинет выдаёт
// одноразовую ссылку, человек жмёт «Начать», и бот узнаёт аккаунт. Пока ждём — раз в 3 секунды
// спрашиваем сервер, не появилась ли привязка.
export function MessengerCard({ onToast }: { onToast: (s: string) => void }) {
  const { t } = useT();
  const { links, tg, refresh } = useAuth();
  const confirm = useConfirm();
  const [bot, setBot] = useState<string | null>(null);
  const [url, setUrl] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const linked = links.includes("telegram");

  useEffect(() => {
    let alive = true;
    api.meta().then((m) => alive && setBot(m.bots?.telegram ?? null)).catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    if (!url || linked) return;
    const until = Date.now() + 15 * 60 * 1000; // столько живёт ссылка
    const id = window.setInterval(() => {
      if (Date.now() > until) setUrl(null);
      else if (!document.hidden) void refresh();
    }, 3000);
    return () => window.clearInterval(id);
  }, [url, linked, refresh]);

  if (!bot) return null;

  const create = async () => {
    setBusy(true);
    try {
      const r = await api.messengerLink("telegram");
      setUrl(r.url);
    } catch (e) {
      onToast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const unlink = async () => {
    if (!(await confirm({ title: t("tg.unlink.confirm"), text: t("tg.unlink.text"), ok: t("tg.unlink"), danger: true }))) return;
    try {
      await api.messengerUnlink("telegram");
      await refresh();
      setUrl(null);
      onToast(t("tg.unlinked"));
    } catch (e) {
      onToast((e as Error).message);
    }
  };

  return (
    <div className="account__card rowcard">
      <span className="rowcard__icon"><Send size={18} aria-hidden /></span>
      <span className="rowcard__text">
        <b>
          Telegram
          {linked && <Check size={16} className="rowcard__done" role="img" aria-label={t("tg.linked.short")} />}
        </b>
        <small>{linked ? t("tg.linked") : url ? t("tg.wait") : t("tg.text")}</small>
      </span>
      {tg?.status === "other" && !linked && <p className="rowcard__note">{t("auth.link.other")}</p>}
      <span className="rowcard__actions">
        {linked ? (
          <button type="button" className="btn btn-ghost btn-sm" onClick={unlink}>{t("tg.unlink")}</button>
        ) : url ? (
          // настоящая ссылка, а не переход из кода: телефон открывает её сразу в приложении Telegram
          <a className="btn btn-primary btn-sm" href={url} target="_blank" rel="noopener" onClick={(e) => { if (openInTelegram(url)) e.preventDefault(); }}>
            {t("tg.open")}
          </a>
        ) : (
          <button type="button" className="btn btn-soft btn-sm" onClick={create} disabled={busy} aria-busy={busy}>{t("tg.link")}</button>
        )}
      </span>
    </div>
  );
}
