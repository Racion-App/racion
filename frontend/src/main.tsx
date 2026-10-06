import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "flag-icons/css/flag-icons.min.css";
import "./styles/app.scss";
import { App } from "./App";
import { initAnalytics, track } from "./lib/analytics";
import { initTelegram } from "./lib/telegram";
import { adoptQueryLang, warmDict } from "./i18n";
import { openCurrentWeek } from "./lib/weeks";

// до первой отрисовки: данные запуска мини-приложения Telegram и язык из ?lang= (так бот открывает сайт)
initTelegram();
adoptQueryLang();
// установленное приложение с идущей неделей открывается сразу на ней, а не на анкете
const toWeek = openCurrentWeek();

import { initOffline } from "./lib/offline";
initAnalytics();
initOffline();
if (toWeek) track("pwa_open_week");
try { window.addEventListener("load", () => setTimeout(() => sessionStorage.removeItem("racion.chunk.reload"), 5000)); } catch { /* приватный режим */ }
// офлайн-оболочка и список покупок: service worker регистрируется сразу, push подключается к нему же
if ("serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {});
  });
}

// Первый кадр — со словарём (warmDict): до этого на экране первый шаг квиза из HTML или крутилка.
// Дольше трёх секунд не ждём — дальше провайдер языка справится сам.
const start = () =>
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
const wait = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));
// Первый шаг из HTML должен успеть показаться: на быстром телефоне приложение готово раньше, чем
// браузер покажет текст шага (он ждёт шрифт), и заменило бы шаг невидимым. Chrome сообщает, когда текст
// подсказки шага (elementtiming="homeshell") действительно на экране, — ждём этого, не дольше секунды.
// Два кадра анимации для этого не годятся: в Chrome они бывают и до первой отрисовки страницы.
// Где Element Timing нет (Safari, Firefox), ждём шрифт и второй кадр анимации — лучшее, что там есть.
const painted = () => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
const presented = () =>
  new Promise<void>((resolve) => {
    if (!PerformanceObserver.supportedEntryTypes?.includes("element")) {
      document.fonts.load("400 16px Inter").then(painted, painted).then(resolve);
      return;
    }
    const po = new PerformanceObserver((list) => {
      if (list.getEntries().some((e) => (e as PerformanceEntry & { identifier?: string }).identifier === "homeshell")) {
        po.disconnect();
        resolve();
      }
    });
    po.observe({ type: "element", buffered: true });
  });
// приложение открывается сразу на неделе: шаг квиза из HTML ждать незачем
const shown = document.getElementById("homeshell") && !toWeek
  ? () => Promise.race([presented(), wait(1000)]).catch(() => {})
  : () => Promise.resolve();
Promise.race([warmDict(), wait(3000)])
  .catch(() => {})
  .then(shown)
  .finally(start);
