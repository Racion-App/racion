import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Bell, BellRing, Send, Smartphone } from "lucide-react";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { track } from "../lib/analytics";
import { isIOS, isStandalone } from "../lib/install";
import { deviceRemind, pushState, pushSubscribe, pushSubscribeDevice, pushUnsubscribeDevice, type PushState } from "../lib/push";
import { readJSON, writeJSON } from "../lib/storage";
import { InstallSheet } from "./InstallSheet";
import { useT } from "../i18n";

// Напоминания со страницы недели. Неделя кончалась, и человек не возвращался: вторую неделю собирали
// единицы, а без аккаунта напомнить было нечем. Без аккаунта подписка привязывается к неделе (сервер
// напоминает по неделям устройства), с аккаунтом — обычная подписка аккаунта с настройками в кабинете.
// Где уведомлений нет (iPhone без установки, запрет в браузере), остаётся бот в Telegram: он напоминает
// по той же неделе.

const LATER_KEY = "racion.remind.later";
const LATER_MS = 14 * 24 * 3600 * 1000;

export function RemindCard({ planId, telegram, onToast }: { planId: string; telegram?: string; onToast: (m: string) => void }) {
  const { t } = useT();
  const { user } = useAuth();
  const [state, setState] = useState<PushState | null>(null);
  const [today, setToday] = useState(() => deviceRemind().today);
  const [busy, setBusy] = useState(false);
  const [install, setInstall] = useState(false);
  const [hidden, setHidden] = useState(() => Date.now() - readJSON<number>(LATER_KEY, 0) < LATER_MS);
  const iosNeedsApp = isIOS() && !isStandalone();

  // браузер подписан, но не этой карточкой (например, раньше через кабинет): без аккаунта считаем, что выключено
  useEffect(() => {
    pushState().then((st) => setState(st === "on" && !user && !deviceRemind().on ? "off" : st)).catch(() => setState("off"));
  }, [user?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  const enable = async () => {
    setBusy(true);
    try {
      let st: PushState;
      if (user) {
        st = await pushSubscribe();
        // часовой пояс устройства — в настройки аккаунта, иначе утреннее напоминание придёт по Москве
        if (st === "on") {
          const r = await api.notify().catch(() => null);
          if (r) await api.setNotify({ ...r.settings, tz: -new Date().getTimezoneOffset() }).catch(() => undefined);
        }
      } else {
        st = await pushSubscribeDevice(planId, today);
      }
      setState(st);
      if (st === "on") {
        onToast(t("remind.done"));
        track("remind_on", { account: !!user });
      } else if (st === "denied") {
        track("remind_denied");
      }
    } catch (e) {
      onToast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const disable = async () => {
    setBusy(true);
    try {
      setState(await pushUnsubscribeDevice());
      onToast(t("remind.stopped"));
      track("remind_off");
    } catch (e) {
      onToast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const toggleToday = async () => {
    const next = !today;
    setToday(next);
    try {
      await pushSubscribeDevice(planId, next);
    } catch (e) {
      setToday(!next);
      onToast((e as Error).message);
    }
  };

  const later = () => {
    writeJSON(LATER_KEY, Date.now());
    setHidden(true);
    track("remind_later");
  };

  if (state === null) return null;

  if (state === "on") {
    return (
      <section className="day remind is-on" aria-label={t("remind.title")}>
        <span className="remind__icon"><BellRing size={18} aria-hidden /></span>
        <div className="remind__text">
          <b>{t("remind.on")}</b>
          {user ? (
            <Link className="remind__link" to="/me">{t("remind.settings")}</Link>
          ) : (
            <>
              <button type="button" className="switch remind__switch" role="switch" aria-checked={today} onClick={toggleToday} disabled={busy}>
                <span className="switch__text">{t("remind.today")}</span>
                <span className="switch__track" aria-hidden><span className="switch__knob" /></span>
              </button>
              <button type="button" className="remind__link" onClick={disable} disabled={busy}>{t("remind.off")}</button>
            </>
          )}
        </div>
      </section>
    );
  }

  if (hidden) return null;
  const canPush = state === "off" && !iosNeedsApp;
  const note = iosNeedsApp ? t("remind.ios") : state === "denied" ? t("remind.denied") : state === "unsupported" ? t("remind.unsupported") : "";
  if (!canPush && !iosNeedsApp && !telegram) return null; // предложить нечего

  return (
    <section className="day remind" aria-label={t("remind.title")}>
      <span className="remind__icon"><Bell size={18} aria-hidden /></span>
      <div className="remind__text">
        <b>{t("remind.title")}</b>
        <p>{t(user ? "remind.lead.user" : "remind.lead")}</p>
        {note && <p className="remind__note">{note}</p>}
        <div className="remind__actions">
          {canPush && (
            <button type="button" className="btn btn-primary btn-sm" onClick={enable} disabled={busy}>
              <Bell size={15} aria-hidden /> {t("remind.enable")}
            </button>
          )}
          {iosNeedsApp && (
            <button type="button" className="btn btn-primary btn-sm" onClick={() => { setInstall(true); track("remind_install"); }}>
              <Smartphone size={15} aria-hidden /> {t("install.button")}
            </button>
          )}
          {telegram && (
            <a className="btn btn-soft btn-sm" href={telegram} target="_blank" rel="noopener" onClick={() => track("remind_tg")}>
              <Send size={15} aria-hidden /> {t("remind.tg")}
            </a>
          )}
          <button type="button" className="remind__link" onClick={later}>{t("remind.later")}</button>
        </div>
      </div>
      {iosNeedsApp && <InstallSheet open={install} onClose={() => setInstall(false)} />}
    </section>
  );
}
