import { useEffect, useState, type ReactNode } from "react";
import { Bell, BellOff, CalendarPlus, ChefHat, CookingPot, MessageCircleQuestion, ShoppingBasket, Smartphone, Sparkles, Sunrise } from "lucide-react";
import { api } from "../lib/api";
import { pushState, pushSubscribe, pushTestResult, pushUnsubscribe, type PushState } from "../lib/push";
import { isIOS, isStandalone } from "../lib/install";
import type { NotifySettings } from "../lib/types";
import { InstallSheet } from "./InstallSheet";
import { Select } from "./Select";
import { intlLocale, useT } from "../i18n";

// Карточка уведомлений в кабинете: разрешение на этом устройстве, потом каждый вид напоминания отдельной
// строкой с переключателем и, где есть, временем. iPhone без установленного приложения пуши не получает,
// поэтому там сначала предлагаем установить.

const HOURS = Array.from({ length: 24 }, (_, h) => ({ value: String(h), label: `${String(h).padStart(2, "0")}:00` }));

function Row({ icon, title, sub, on, onToggle, extra }: { icon: ReactNode; title: string; sub: string; on: boolean; onToggle: () => void; extra?: ReactNode }) {
  return (
    <div className={"nrow" + (on ? " is-on" : "")}>
      <span className="nrow__icon">{icon}</span>
      <span className="nrow__text">
        <b>{title}</b>
        <small>{sub}</small>
        {on && extra && <span className="nrow__extra">{extra}</span>}
      </span>
      <button type="button" className="switch switch--bare" role="switch" aria-checked={on} aria-label={title} onClick={onToggle}>
        <span className="switch__track" aria-hidden><span className="switch__knob" /></span>
      </button>
    </div>
  );
}

export function NotifyCard({ onToast }: { onToast: (m: string) => void }) {
  const { t, lang } = useT();
  const [state, setState] = useState<PushState>("off");
  const [s, setS] = useState<NotifySettings | null>(null);
  const [devices, setDevices] = useState(0);
  const [busy, setBusy] = useState(false);
  const [install, setInstall] = useState(false);
  const iosNeedsApp = isIOS() && !isStandalone();
  const days = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(2026, 8, 6 + i); // 6 сентября 2026 — воскресенье
    const w = d.toLocaleDateString(intlLocale(lang), { weekday: "long" });
    return w.charAt(0).toUpperCase() + w.slice(1);
  });

  useEffect(() => {
    Promise.all([pushState(), api.notify()])
      .then(async ([st, r]) => {
        setS({ ...r.settings, tz: -new Date().getTimezoneOffset() });
        setDevices(r.devices ?? 0);
        // браузер подписан, а сервер это устройство не знает (сменились ключи или сервер): переподписываемся молча
        if (st === "on" && (r.devices ?? 0) === 0) {
          try {
            st = await pushSubscribe();
            setDevices(st === "on" ? 1 : 0);
          } catch {
            // покажем состояние как есть
          }
        }
        setState(st);
      })
      .catch(() => { pushState().then(setState); setS(null); });
  }, []);

  const save = async (next: NotifySettings) => {
    setS(next);
    try {
      await api.setNotify(next);
    } catch (e) {
      onToast((e as Error).message);
    }
  };

  const toggle = async () => {
    setBusy(true);
    try {
      const st = state === "on" ? await pushUnsubscribe() : await pushSubscribe();
      setState(st);
      if (st === "on" && s) await api.setNotify(s); // часовой пояс устройства
      if (st === "denied") onToast(t("notify.denied"));
      setDevices((n) => (st === "on" ? Math.max(1, n + 1) : Math.max(0, n - 1)));
    } catch (e) {
      onToast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  // проверка: если push-сервис отверг старую подписку, сервер её удалил — переподписываемся и шлём ещё раз.
  // Потом ждём до 8 секунд: service worker сообщает, что пуш дошёл, а getNotifications показывает, вывела ли
  // его система. Так человек видит, где обрыв: сеть, браузер или настройки уведомлений на устройстве.
  const test = async () => {
    setBusy(true);
    try {
      try {
        await api.notifyTest();
      } catch {
        const st = await pushSubscribe();
        setState(st);
        await api.notifyTest();
      }
      onToast(t(await pushTestResult()));
    } catch (e) {
      onToast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const at = (value: number, onChange: (v: number) => void, label: string) => (
    <>
      {t("notify.at")} <Select aria-label={label} className="select--sm" value={String(value)} onChange={(v) => onChange(Number(v))} options={HOURS} />
    </>
  );

  return (
    <div className="account__card notify" aria-label={t("notify.title")}>
      <div className="notify__head">
        <span className="notify__title"><Bell size={16} aria-hidden /> {t("notify.title")}</span>
        {devices > 0 && <span className="notify__on">{t("notify.devices", { n: devices })}</span>}
      </div>
      <p className="quiz__hint">{t("notify.lead")}</p>

      {state === "unsupported" && !iosNeedsApp && <p className="notify__note">{t("notify.unsupported")}</p>}
      {iosNeedsApp && state !== "on" && (
        <div className="notify__ios">
          <p>{t("notify.ios.pwa")}</p>
          <button type="button" className="btn btn-primary" onClick={() => setInstall(true)}><Smartphone size={16} aria-hidden /> {t("install.button")}</button>
          <InstallSheet open={install} onClose={() => setInstall(false)} />
        </div>
      )}
      {state === "denied" && <p className="notify__note">{t("notify.denied.how")}</p>}
      {(state === "off" || state === "on") && !(iosNeedsApp && state === "off") && (
        <div className="notify__perm">
          <button type="button" className={"btn " + (state === "on" ? "btn-soft" : "btn-primary")} onClick={toggle} disabled={busy}>
            {state === "on" ? <BellOff size={16} aria-hidden /> : <Bell size={16} aria-hidden />} {state === "on" ? t("notify.disable") : t("notify.enable")}
          </button>
          {state === "on" && (
            <button type="button" className="btn btn-ghost" onClick={test} disabled={busy}>
              {t("notify.test")}
            </button>
          )}
        </div>
      )}

      {s && (
        <div className={"notify__list" + (state === "on" ? "" : " is-muted")} aria-disabled={state !== "on"}>
          <Row icon={<Sunrise size={18} aria-hidden />} title={t("notify.today")} sub={t("notify.today.sub")} on={s.today} onToggle={() => save({ ...s, today: !s.today })} extra={at(s.todayHour, (v) => save({ ...s, todayHour: v }), t("notify.today"))} />
          <Row icon={<ChefHat size={18} aria-hidden />} title={t("notify.prep")} sub={t("notify.prep.sub")} on={s.prep} onToggle={() => save({ ...s, prep: !s.prep })} extra={at(s.prepHour, (v) => save({ ...s, prepHour: v }), t("notify.prep"))} />
          <Row icon={<CookingPot size={18} aria-hidden />} title={t("notify.prepday")} sub={t("notify.prepday.sub")} on={s.prepDay} onToggle={() => save({ ...s, prepDay: !s.prepDay })} />
          <Row
            icon={<ShoppingBasket size={18} aria-hidden />}
            title={t("notify.shop")}
            sub={t("notify.shop.sub")}
            on={s.shopHour >= 0}
            onToggle={() => save({ ...s, shopHour: s.shopHour >= 0 ? -1 : 12 })}
            extra={
              <>
                <Select aria-label={t("notify.shop.day")} className="select--sm" value={String(s.shopDay)} onChange={(v) => save({ ...s, shopDay: Number(v) })} options={days.map((d, i) => ({ value: String(i), label: d }))} />
                {at(Math.max(0, s.shopHour), (v) => save({ ...s, shopHour: v }), t("notify.shop.hour"))}
              </>
            }
          />
          <Row icon={<CalendarPlus size={18} aria-hidden />} title={t("notify.week")} sub={t("notify.week.sub")} on={s.week} onToggle={() => save({ ...s, week: !s.week })} />
          <Row icon={<MessageCircleQuestion size={18} aria-hidden />} title={t("notify.ask")} sub={t("notify.ask.sub")} on={!s.noAsk} onToggle={() => save({ ...s, noAsk: !s.noAsk })} />
          <Row icon={<Sparkles size={18} aria-hidden />} title={t("notify.digest")} sub={t("notify.digest.sub")} on={s.digest} onToggle={() => save({ ...s, digest: !s.digest })} />
        </div>
      )}
    </div>
  );
}
