// Собственная аналитика + Яндекс.Метрика (если задан VITE_METRIKA_ID).
// Анонимный sid живёт в localStorage; события копятся и уходят пачкой.

const SID_KEY = "racion.sid";
const METRIKA_ID = import.meta.env.VITE_METRIKA_ID as string | undefined;

type Ev = { name: string; props?: Record<string, unknown>; ts: number };

function sid(): string {
  try {
    let v = localStorage.getItem(SID_KEY);
    if (!v) {
      v = typeof crypto.randomUUID === "function" ? crypto.randomUUID() : Math.random().toString(36).slice(2) + Date.now().toString(36);
      localStorage.setItem(SID_KEY, v);
    }
    return v;
  } catch {
    return "anon";
  }
}

const queue: Ev[] = [];
let timer: number | undefined;

function flush(useBeacon = false) {
  if (queue.length === 0) return;
  const body = JSON.stringify({ sid: sid(), events: queue.splice(0, 50) });
  if (useBeacon && navigator.sendBeacon) {
    navigator.sendBeacon("/api/events", new Blob([body], { type: "application/json" }));
    return;
  }
  fetch("/api/events", { method: "POST", headers: { "Content-Type": "application/json" }, body, keepalive: true }).catch(() => {});
}

export function track(name: string, props?: Record<string, unknown>) {
  queue.push({ name, props, ts: Date.now() });
  if (queue.length >= 10) flush();
  else {
    if (timer) window.clearTimeout(timer);
    timer = window.setTimeout(() => flush(), 1500);
  }
  const w = window as unknown as { ym?: (id: number, method: string, ...args: unknown[]) => void };
  if (METRIKA_ID && w.ym) w.ym(Number(METRIKA_ID), "reachGoal", name, props);
}

// Ошибки браузера уходят в аналитику как js_error: их видно в админке. Стек режем, чтобы влезть в лимит события.
// Шум, а не ошибки: браузер пропустил view transition (React запускает их сам при смене шага), сеть оборвалась
const NOISE = [/Transition was skipped/i, /Failed to fetch/i, /Load failed/i, /NetworkError/i, /AbortError/i];
function reportError(message: string, stack?: string) {
  if (NOISE.some((re) => re.test(message))) return;
  track("js_error", { message: message.slice(0, 300), stack: (stack ?? "").slice(0, 1500), url: location.pathname + location.search, ua: navigator.userAgent.slice(0, 200) });
}

export function initAnalytics() {
  window.addEventListener("error", (e) => reportError(e.message || String(e.error), e.error?.stack));
  window.addEventListener("unhandledrejection", (e) => {
    const r = e.reason as { message?: string; stack?: string } | undefined;
    reportError(r?.message ?? String(e.reason), r?.stack);
  });
  window.addEventListener("pagehide", () => flush(true));
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") flush(true);
  });
  if (!METRIKA_ID) return;
  const w = window as unknown as Record<string, unknown>;
  if (w.ym) return;
  const ymq: unknown[] = [];
  const ym = Object.assign((...args: unknown[]) => void ymq.push(args), { a: ymq, l: Date.now() });
  w.ym = ym;
  const s = document.createElement("script");
  s.async = true;
  s.src = "https://mc.yandex.ru/metrika/tag.js";
  document.head.appendChild(s);
  ym(Number(METRIKA_ID), "init", { clickmap: true, trackLinks: true, accurateTrackBounce: true, webvisor: false });
}
