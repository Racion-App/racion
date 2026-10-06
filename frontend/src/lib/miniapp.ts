// Мини-приложение мессенджера: сайт открыт внутри Telegram или MAX. Мессенджер передаёт подписанные
// данные человека в адресе после # (Telegram — tgWebAppData, MAX — WebAppData). Запоминаем их сразу
// при загрузке, пока роутер не сменил адрес, — по ним сервер узнаёт человека без пароля. SDK
// мессенджера грузим только внутри него: обычному посетителю сторонний скрипт не нужен.
import { applyTheme } from "./theme";

export type MiniPlatform = "telegram" | "max";

type BackButton = { show(): void; hide(): void; onClick(cb: () => void): void; offClick(cb: () => void): void };

// MiniSdk — общее у SDK Telegram (window.Telegram.WebApp) и моста MAX (window.WebApp); остальное есть
// только у одного из них.
export type MiniSdk = {
  ready(): void;
  BackButton: BackButton;
  HapticFeedback?: { selectionChanged(): void };
  // Telegram
  initData?: string;
  colorScheme?: "light" | "dark";
  expand?(): void;
  setHeaderColor?(color: string): void;
  setBackgroundColor?(color: string): void;
  setBottomBarColor?(color: string): void;
  onEvent?(event: string, cb: () => void): void;
  openTelegramLink?(url: string): void;
  // MAX
  openMaxLink?(url: string): void;
};

declare global {
  interface Window {
    Telegram?: { WebApp?: MiniSdk };
    WebApp?: MiniSdk;
  }
}

const KEY = "racion.miniapp"; // платформа и данные запуска на сессию вкладки: после переходов внутри приложения хеша в адресе уже нет
const SDK: Record<MiniPlatform, string> = {
  telegram: "https://telegram.org/js/telegram-web-app.js?59",
  max: "https://st.max.ru/js/max-web-app.js",
};

let platform: MiniPlatform | "" = "";
let initData = "";

// captureMiniApp — до первой отрисовки: данные запуска из адреса или из прошлых переходов этой сессии.
export function captureMiniApp() {
  try {
    const hash = new URLSearchParams(location.hash.slice(1));
    const tg = hash.get("tgWebAppData");
    const mx = hash.get("WebAppData");
    if (tg) {
      platform = "telegram";
      initData = tg;
      // SDK Telegram читает параметры из адреса, а грузится позже, когда адрес мог смениться: кладём их
      // туда, где он ищет сохранённые
      const params: Record<string, string> = {};
      hash.forEach((v, k) => (params[k] = v));
      sessionStorage.setItem("__telegram__initParams", JSON.stringify(params));
      // тема мессенджера до первой отрисовки: SDK ещё не загружен, а цвет фона уже в параметрах
      const bg = (JSON.parse(hash.get("tgWebAppThemeParams") ?? "{}") as { bg_color?: string }).bg_color ?? "";
      if (/^#[0-9a-f]{6}$/i.test(bg)) applyTheme(luma(bg) < 128 ? "dark" : "light");
    } else if (mx) {
      // мост MAX сам сохраняет свои параметры и находит их в адресе первой загрузки
      platform = "max";
      initData = mx;
    }
    if (platform) {
      sessionStorage.setItem(KEY, JSON.stringify({ platform, initData }));
    } else {
      const saved = JSON.parse(sessionStorage.getItem(KEY) ?? "null") as { platform?: MiniPlatform; initData?: string } | null;
      if (saved?.platform && saved.initData) {
        platform = saved.platform;
        initData = saved.initData;
      }
    }
  } catch {
    // без хранилища: мини-приложение работает как обычный сайт
  }
  if (platform) document.documentElement.classList.add("in-miniapp", platform === "telegram" ? "in-tg" : "in-max");
}

export const inMiniApp = () => platform !== "";
export const inTelegram = () => platform === "telegram";
export const miniPlatform = () => platform;
export const miniInitData = () => initData;

let loading: Promise<MiniSdk | null> | null = null;

// miniApp — SDK мессенджера; вне мини-приложения сразу null.
export function miniApp(): Promise<MiniSdk | null> {
  if (!platform) return Promise.resolve(null);
  const p = platform;
  const sdk = () => (p === "telegram" ? window.Telegram?.WebApp : window.WebApp) ?? null;
  if (!loading) {
    loading = new Promise((resolve) => {
      if (sdk()) return resolve(sdk());
      const s = document.createElement("script");
      s.src = SDK[p];
      s.async = true;
      s.onload = () => resolve(sdk());
      s.onerror = () => resolve(null);
      document.head.appendChild(s);
    });
  }
  return loading;
}

// initMiniApp — мини-приложение готово к показу. Telegram — на весь экран, цвета шапки и фона под
// страницу, тема как у мессенджера; у моста MAX настроек темы нет, остаётся тема сайта.
export function initMiniApp() {
  captureMiniApp();
  void miniApp().then((app) => {
    if (!app) return;
    app.ready();
    if (platform !== "telegram") return;
    app.expand?.();
    const paint = () => {
      if (app.colorScheme) applyTheme(app.colorScheme); // без записи в хранилище: на сайте у человека может быть своя тема
      const bg = hexColor(getComputedStyle(document.body).backgroundColor);
      if (!bg) return;
      app.setHeaderColor?.(bg);
      app.setBackgroundColor?.(bg);
      app.setBottomBarColor?.(bg);
    };
    paint();
    app.onEvent?.("themeChanged", paint);
  });
}

// haptic — лёгкий отклик под пальцем при отметке покупки; вне мини-приложения ничего.
export function haptic() {
  try {
    (platform === "telegram" ? window.Telegram?.WebApp : platform === "max" ? window.WebApp : undefined)?.HapticFeedback?.selectionChanged();
  } catch {
    // отклика нет на компьютере: не беда
  }
}

// openInMessenger — ссылку на бота внутри мини-приложения открывает сам мессенджер, а не встроенный
// браузер: t.me — в Telegram, max.ru — в MAX.
export function openInMessenger(url: string): boolean {
  if (platform === "telegram" && window.Telegram?.WebApp?.openTelegramLink && /^https:\/\/t\.me\//.test(url)) {
    window.Telegram.WebApp.openTelegramLink(url);
    return true;
  }
  if (platform === "max" && window.WebApp?.openMaxLink && /^https:\/\/max\.ru\//.test(url)) {
    window.WebApp.openMaxLink(url);
    return true;
  }
  return false;
}

function luma(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  return ((n >> 16) & 255) * 0.299 + ((n >> 8) & 255) * 0.587 + (n & 255) * 0.114;
}

function hexColor(rgb: string): string {
  const m = rgb.match(/\d+(\.\d+)?/g);
  if (!m || m.length < 3) return "";
  return "#" + m.slice(0, 3).map((v) => Math.round(Number(v)).toString(16).padStart(2, "0")).join("");
}
