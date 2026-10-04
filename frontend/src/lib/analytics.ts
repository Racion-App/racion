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
  metrika("reachGoal", name, props);
}

// Счётчик Метрики стартует после первого касания или через 5 секунд (index.html): до этого window.ym нет.
// Цели и просмотры этого времени ждут его в очереди, а не теряются; через минуту без счётчика
// (отказ от аналитики, не racion.app) очередь выбрасывается.
type Ym = (id: number, method: string, ...args: unknown[]) => void;
const pendingYm: unknown[][] = [];
let ymWait = 0;
function metrika(method: string, ...args: unknown[]) {
  if (!METRIKA_ID) return;
  const ym = (window as unknown as { ym?: Ym }).ym;
  if (ym) {
    ym(Number(METRIKA_ID), method, ...args);
    return;
  }
  if (pendingYm.length < 50) pendingYm.push([method, ...args]);
  if (ymWait) return;
  const t0 = Date.now();
  const tick = () => {
    const f = (window as unknown as { ym?: Ym }).ym;
    if (f) {
      ymWait = 0;
      const items = pendingYm.splice(0);
      // текущую страницу счётчик посчитал сам при запуске — последний переход на неё второй раз не шлём
      const last = items.map((x) => x[0]).lastIndexOf("hit");
      items.forEach(([m, ...a], i) => {
        if (!(i === last && a[0] === location.href)) f(Number(METRIKA_ID), m as string, ...a);
      });
    } else if (Date.now() - t0 < 60_000) ymWait = window.setTimeout(tick, 1000);
    else {
      ymWait = 0;
      pendingYm.length = 0;
    }
  };
  ymWait = window.setTimeout(tick, 1000);
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
  // сам счётчик — inline в index.html (так его видит проверка Метрики); здесь просмотры при переходах внутри
  // приложения: они идут через pushState, и Метрика сама их не видит. Раньше перехват ставился, только если
  // счётчик уже загружен, а он грузится позже, — шаги квиза и страница недели в Метрику почти не попадали.
  const push = history.pushState.bind(history);
  history.pushState = (...args: Parameters<History["pushState"]>) => {
    push(...args);
    try {
      metrika("hit", location.href);
    } catch {
      /* счётчик сломался — переход важнее */
    }
  };
}
