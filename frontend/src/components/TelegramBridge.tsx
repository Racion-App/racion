import { useEffect, useRef } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { inMiniApp, miniApp } from "../lib/miniapp";

// TelegramBridge — мини-приложение (Telegram, MAX) и роутер: кнопка «Назад» в шапке мессенджера, когда
// есть куда вернуться (иначе там «Закрыть»), и переход по параметру запуска (неделя из ссылки startapp).
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
    if (!inMiniApp()) return;
    let alive = true;
    let off: (() => void) | null = null;
    void miniApp().then((app) => {
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
