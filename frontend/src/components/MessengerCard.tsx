import { useEffect, useState } from "react";
import { Check, MessageCircle, Send } from "lucide-react";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { miniPlatform, openInMessenger, type MiniPlatform } from "../lib/miniapp";
import { useConfirm } from "./Confirm";
import { useT } from "../i18n";

// MessengerCard — привязка мессенджера (Telegram, MAX) в кабинете. Привязанный аккаунт входит в
// мини-приложении без пароля, а бот присылает списки недель этого аккаунта. Привязка идёт через бота:
// кабинет выдаёт одноразовую ссылку, человек жмёт «Начать», и бот узнаёт аккаунт. Пока ждём — раз
// в 3 секунды спрашиваем сервер, не появилась ли привязка. Карточка видна, только если бот подключён.

const NAME: Record<MiniPlatform, string> = { telegram: "Telegram", max: "MAX" };
// тексты, где назван мессенджер, у каждого свои; «Привязать» и «Отвязать» общие
const KEY: Record<MiniPlatform, string> = { telegram: "tg", max: "max" };

export function MessengerCard({ platform, onToast }: { platform: MiniPlatform; onToast: (s: string) => void }) {
  const { t } = useT();
  const { links, tg, refresh } = useAuth();
  const confirm = useConfirm();
  const [bot, setBot] = useState<string | null>(null);
  const [url, setUrl] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const linked = links.includes(platform);
  const k = KEY[platform];

  useEffect(() => {
    let alive = true;
    api.meta().then((m) => alive && setBot(m.bots?.[platform] ?? null)).catch(() => {});
    return () => {
      alive = false;
    };
  }, [platform]);

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
      const r = await api.messengerLink(platform);
      setUrl(r.url);
    } catch (e) {
      onToast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const unlink = async () => {
    if (!(await confirm({ title: t(`${k}.unlink.confirm`), text: t(`${k}.unlink.text`), ok: t("tg.unlink"), danger: true }))) return;
    try {
      await api.messengerUnlink(platform);
      await refresh();
      setUrl(null);
      onToast(t(`${k}.unlinked`));
    } catch (e) {
      onToast((e as Error).message);
    }
  };

  return (
    <div className="account__card rowcard">
      <span className="rowcard__icon">{platform === "max" ? <MessageCircle size={18} aria-hidden /> : <Send size={18} aria-hidden />}</span>
      <span className="rowcard__text">
        <b>
          {NAME[platform]}
          {linked && <Check size={16} className="rowcard__done" role="img" aria-label={t(`${k}.linked.short`)} />}
        </b>
        <small>{linked ? t(`${k}.linked`) : url ? t(`${k}.wait`) : t(`${k}.text`)}</small>
      </span>
      {tg?.status === "other" && miniPlatform() === platform && !linked && <p className="rowcard__note">{t("auth.link.other")}</p>}
      <span className="rowcard__actions">
        {linked ? (
          <button type="button" className="btn btn-ghost btn-sm" onClick={unlink}>{t("tg.unlink")}</button>
        ) : url ? (
          // настоящая ссылка, а не переход из кода: телефон открывает её сразу в приложении мессенджера
          <a className="btn btn-primary btn-sm" href={url} target="_blank" rel="noopener" onClick={(e) => { if (openInMessenger(url)) e.preventDefault(); }}>
            {t(`${k}.open`)}
          </a>
        ) : (
          <button type="button" className="btn btn-soft btn-sm" onClick={create} disabled={busy} aria-busy={busy}>{t("tg.link")}</button>
        )}
      </span>
    </div>
  );
}
