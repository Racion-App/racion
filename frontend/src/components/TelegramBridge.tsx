import { useEffect, useRef } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { inTelegram, telegram } from "../lib/telegram";

// TelegramBridge — мини-приложение и роутер: кнопка «Назад» в шапке Telegram, когда есть куда
// вернуться (иначе там «Закрыть»), и переход по параметру запуска (неделя из ссылки startapp).
export function TelegramBridge() {
  const loc = useLocation();
  const nav = useNavigate();
  const { tg } = useAuth();

  const opened = useRef(false);
  useEffect(() => {
    if (!tg?.open || opened.current) return;
    opened.current = true;
    // только со стартовой страницы: если человек уже ушёл дальше, не уводим его
    if (/^(\/[a-z]{2})?\/?$/.test(loc.pathname)) nav(tg.open, { replace: true });
  }, [tg, loc.pathname, nav]);

  useEffect(() => {
    if (!inTelegram()) return;
    let alive = true;
    let off: (() => void) | null = null;
    void telegram().then((app) => {
      if (!app || !alive) return;
      const depth = (window.history.state as { idx?: number } | null)?.idx ?? 0;
      if (depth > 0) {
        const back = () => nav(-1);
        app.BackButton.onClick(back);
        app.BackButton.show();
        off = () => app.BackButton.offClick(back);
      } else {
        app.BackButton.hide();
      }
    });
    return () => {
      alive = false;
      off?.();
    };
  }, [loc.key, nav]);

  return null;
}
