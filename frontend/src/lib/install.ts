// Установка PWA: ловим beforeinstallprompt (Chrome/Edge/Android), распознаём iOS (там только через
// «Поделиться → На экран Домой») и уже установленное приложение (display-mode: standalone).

type BeforeInstallPromptEvent = Event & { prompt: () => Promise<void>; userChoice: Promise<{ outcome: "accepted" | "dismissed" }> };

let deferred: BeforeInstallPromptEvent | null = null;
const listeners = new Set<() => void>();

if (typeof window !== "undefined") {
  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault();
    deferred = e as BeforeInstallPromptEvent;
    listeners.forEach((f) => f());
  });
  window.addEventListener("appinstalled", () => {
    deferred = null;
    try {
      localStorage.setItem("racion.pwa.installed", "1");
    } catch {
      // приватный режим
    }
    listeners.forEach((f) => f());
  });
}

export function onInstallChange(f: () => void): () => void {
  listeners.add(f);
  return () => {
    listeners.delete(f);
  };
}

export function isStandalone(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia("(display-mode: standalone)").matches || (navigator as Navigator & { standalone?: boolean }).standalone === true;
}

export function isIOS(): boolean {
  if (typeof navigator === "undefined") return false;
  const ua = navigator.userAgent;
  return /iPhone|iPad|iPod/.test(ua) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
}

// iosMajor — версия iOS из user agent (в iOS 26 Safari спрятал «Поделиться» в меню «…», инструкция другая).
// iOS 26 в user agent по-прежнему пишет «OS 18_7», настоящая версия только в «Version/26.x» — берём большее.
export function iosMajor(): number {
  if (typeof navigator === "undefined") return 0;
  const ua = navigator.userAgent;
  const os = /OS (\d+)_/.exec(ua);
  const ver = /Version\/(\d+)/.exec(ua);
  return Math.max(os ? Number(os[1]) : 0, ver ? Number(ver[1]) : 0);
}

export function isMobile(): boolean {
  if (typeof navigator === "undefined") return false;
  return /Android|iPhone|iPad|iPod|Mobile/.test(navigator.userAgent) || (navigator.maxTouchPoints > 1 && window.innerWidth < 900);
}

// canPrompt — браузер готов показать свой диалог установки
export function canPrompt(): boolean {
  return deferred !== null;
}

export async function promptInstall(): Promise<"accepted" | "dismissed" | "unavailable"> {
  if (!deferred) return "unavailable";
  const ev = deferred;
  deferred = null;
  await ev.prompt();
  const { outcome } = await ev.userChoice;
  listeners.forEach((f) => f());
  return outcome;
}

// platform — какую инструкцию показывать
export function installPlatform(): "ios" | "android" | "desktop" {
  if (isIOS()) return "ios";
  if (isMobile()) return "android";
  return "desktop";
}

const DISMISS_KEY = "racion.pwa.dismissed";

// shouldSuggest — предлагать ли установку сейчас: телефон, не установлено, не отказывались последние 14 дней
export function shouldSuggest(): boolean {
  if (isStandalone() || !isMobile()) return false;
  try {
    if (localStorage.getItem("racion.pwa.installed") === "1") return false;
    const t = Number(localStorage.getItem(DISMISS_KEY) || 0);
    if (Date.now() - t < 14 * 24 * 3600 * 1000) return false;
  } catch {
    // приватный режим
  }
  return true;
}

export function dismissSuggest() {
  try {
    localStorage.setItem(DISMISS_KEY, String(Date.now()));
  } catch {
    // приватный режим
  }
}
