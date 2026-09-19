import { useEffect, useState } from "react";
import { Bell, BellOff } from "lucide-react";
import { api } from "../lib/api";
import { pushState, pushSubscribe, pushUnsubscribe, type PushState } from "../lib/push";
import type { NotifySettings } from "../lib/types";
import { Select } from "./Select";
import { intlLocale, useT } from "../i18n";

// Карточка уведомлений в кабинете: включить на этом устройстве, день и час похода в магазин,
// вечерняя сводка «завтра готовим», воскресное «собрать неделю».


export function NotifyCard({ onToast }: { onToast: (m: string) => void }) {
  const { t, lang } = useT();
  const [state, setState] = useState<PushState>("off");
  const [s, setS] = useState<NotifySettings | null>(null);
  const [busy, setBusy] = useState(false);
  // названия дней недели из Intl по языку интерфейса (воскресенье — первый, как в настройках)
  const days = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(2026, 8, 6 + i); // 6 сентября 2026 — воскресенье
    const s = d.toLocaleDateString(intlLocale(lang), { weekday: "long" });
    return s.charAt(0).toUpperCase() + s.slice(1);
  });

  useEffect(() => {
    pushState().then(setState);
    api.notify().then((r) => setS({ ...r.settings, tz: -new Date().getTimezoneOffset() })).catch(() => setS(null));
  }, []);

  const save = async (next: NotifySettings) => {
    setS(next);
    try {
      await api.setNotify(next);
      onToast(t("notify.saved"));
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
    } catch (e) {
      onToast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="account__card notify" aria-label={t("notify.title")}>
      <div className="notify__head">
        <span className="notify__title">
          <Bell size={16} aria-hidden /> {t("notify.title")}
        </span>
        {state === "on" && <span className="notify__on">{t("notify.on")}</span>}
      </div>
      <p className="quiz__hint">{t("notify.hint")}</p>
      {state === "unsupported" && <p className="notify__note">{t("notify.unsupported")}</p>}
      {state === "denied" && <p className="notify__note">{t("notify.denied")}</p>}
      {(state === "off" || state === "on") && (
        <button type="button" className="btn btn-soft" onClick={toggle} disabled={busy}>
          {state === "on" ? <BellOff size={16} aria-hidden /> : <Bell size={16} aria-hidden />} {state === "on" ? t("notify.disable") : t("notify.enable")}
        </button>
      )}
      {s && state === "on" && (
        <div className="notify__settings">
          <div className="notify__row">
            <span>{t("notify.shop")}</span>
            <div className="notify__pair">
              <Select aria-label={t("notify.shop.day")} value={String(s.shopDay)} onChange={(v) => save({ ...s, shopDay: Number(v) })} options={days.map((d, i) => ({ value: String(i), label: d }))} />
              <Select aria-label={t("notify.shop.hour")} value={String(s.shopHour)} onChange={(v) => save({ ...s, shopHour: Number(v) })} options={Array.from({ length: 16 }, (_, i) => i + 7).map((h) => ({ value: String(h), label: `${String(h).padStart(2, "0")}:00` }))} />
            </div>
          </div>
          <button type="button" className="switch" role="switch" aria-checked={s.prep} onClick={() => save({ ...s, prep: !s.prep })}>
            <span className="switch__text">{t("notify.prep")}</span>
            <span className="switch__track" aria-hidden>
              <span className="switch__knob" />
            </span>
          </button>
          <button type="button" className="switch" role="switch" aria-checked={s.week} onClick={() => save({ ...s, week: !s.week })}>
            <span className="switch__text">{t("notify.week")}</span>
            <span className="switch__track" aria-hidden>
              <span className="switch__knob" />
            </span>
          </button>
          <button type="button" className="switch" role="switch" aria-checked={!s.noAsk} onClick={() => save({ ...s, noAsk: !s.noAsk })} title={t("notify.ask.hint")}>
            <span className="switch__text">{t("notify.ask")}</span>
            <span className="switch__track" aria-hidden>
              <span className="switch__knob" />
            </span>
          </button>
          <button type="button" className="btn btn-ghost" onClick={() => api.notifyTest().then(() => onToast(t("notify.test.sent"))).catch((e: Error) => onToast(e.message))}>
            {t("notify.test")}
          </button>
        </div>
      )}
    </div>
  );
}
