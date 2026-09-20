// Обновление PWA: сравниваем метку сборки, вшитую в бандл, с /version.json на сервере. Проверяем через
// полминуты после запуска, затем каждые 15 минут и при возврате на вкладку (не чаще раза в 2 минуты).
// Если версии разошлись — приложение показывает полосу «Вышло обновление»; applyUpdate чистит кэши
// service worker и перезагружает страницу, чтобы скрипты и стили точно пришли с сервера.

declare const __BUILD__: string;

export const BUILD = typeof __BUILD__ === "string" ? __BUILD__ : "dev";

const FIRST_MS = 30_000;
const FIRST_STANDALONE_MS = 3_000; // установленное приложение живёт неделями без перезагрузки: проверяем сразу
const EVERY_MS = 15 * 60_000;
const MIN_GAP_MS = 2 * 60_000;

let last = 0;
let latest = "";

async function fetchBuild(): Promise<string> {
  const r = await fetch("/version.json?t=" + Date.now(), { cache: "no-store" });
  if (!r.ok) throw new Error(String(r.status));
  const j = (await r.json()) as { build?: string };
  return j.build ?? "";
}

// startUpdateCheck запускает проверки; onUpdate вызывается один раз, когда на сервере другая сборка
export function startUpdateCheck(onUpdate: (build: string) => void): () => void {
  if (typeof window === "undefined" || BUILD === "dev") return () => undefined;
  let stopped = false;
  const check = async () => {
    if (stopped || Date.now() - last < MIN_GAP_MS || !navigator.onLine) return;
    last = Date.now();
    try {
      const b = await fetchBuild();
      if (b && b !== BUILD && b !== latest) {
        latest = b;
        onUpdate(b);
      }
    } catch {
      // без сети или сервер на выкладке — проверим в следующий раз
    }
  };
  const standalone = window.matchMedia("(display-mode: standalone)").matches || (navigator as Navigator & { standalone?: boolean }).standalone === true;
  const t0 = window.setTimeout(check, standalone ? FIRST_STANDALONE_MS : FIRST_MS);
  const iv = window.setInterval(check, EVERY_MS);
  const vis = () => { if (document.visibilityState === "visible") check(); };
  document.addEventListener("visibilitychange", vis);
  // новый service worker активировался поверх старого: сборка точно новее
  const onMsg = (e: MessageEvent) => { if (e.data?.type === "update" && !stopped && !latest) { latest = "sw"; onUpdate("sw"); } };
  navigator.serviceWorker?.addEventListener("message", onMsg);
  return () => { stopped = true; window.clearTimeout(t0); window.clearInterval(iv); document.removeEventListener("visibilitychange", vis); navigator.serviceWorker?.removeEventListener("message", onMsg); };
}

// applyUpdate: свежий service worker, пустые кэши, перезагрузка
export async function applyUpdate(): Promise<void> {
  try {
    const reg = await navigator.serviceWorker?.getRegistration("/");
    await reg?.update();
  } catch {
    // service worker не обязателен
  }
  await clearCaches();
  location.reload();
}

async function clearCaches(): Promise<void> {
  try {
    const keys = await caches.keys();
    await Promise.all(keys.map((k) => caches.delete(k)));
  } catch {
    // приватный режим
  }
}

// resetApp: кнопка «Обновить» в кабинете. Обновляет service worker (не снимает: вместе с ним пропала бы
// push-подписка устройства), чистит кэши и грузит страницу заново мимо HTTP-кэша (метка в адресе).
// Вход и настройки не трогаем: они в cookie и на сервере.
export async function resetApp(): Promise<void> {
  try {
    const reg = await navigator.serviceWorker?.getRegistration("/");
    await reg?.update();
  } catch {
    // без service worker
  }
  await clearCaches();
  const u = new URL(location.href);
  u.searchParams.set("v", String(Date.now()));
  location.replace(u.toString());
}
