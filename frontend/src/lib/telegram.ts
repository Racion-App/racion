// Мини-приложение Telegram: сайт открыт внутри мессенджера. Telegram передаёт подписанные данные человека
// в адресе (#tgWebAppData=…). Запоминаем их сразу при загрузке, пока роутер не сменил адрес, — по ним
// сервер узнаёт человека без пароля. SDK Telegram грузим только внутри мессенджера: обычному посетителю
// сторонний скрипт не нужен.
import { applyTheme } from "./theme";

export type TgWebApp = {
  initData: string;
  colorScheme: "light" | "dark";
  ready(): void;
  expand(): void;
  setHeaderColor(color: string): void;
  setBackgroundColor(color: string): void;
  setBottomBarColor?(color: string): void;
  onEvent(event: string, cb: () => void): void;
  openTelegramLink(url: string): void;
  BackButton: { show(): void; hide(): void; onClick(cb: () => void): void; offClick(cb: () => void): void };
  HapticFeedback?: { selectionChanged(): void; notificationOccurred(type: "success" | "error" | "warning"): void };
};

declare global {
  interface Window {
    Telegram?: { WebApp?: TgWebApp };
  }
}

const KEY = "racion.tg"; // initData на сессию вкладки: после переходов внутри приложения хеша в адресе уже нет
const SDK = "https://telegram.org/js/telegram-web-app.js?59";

let initData = "";

// captureTelegram — до первой отрисовки: данные запуска из адреса или из прошлых переходов этой сессии.
export function captureTelegram() {
  try {
    const hash = new URLSearchParams(location.hash.slice(1));
    const data = hash.get("tgWebAppData");
    if (data) {
      initData = data;
      sessionStorage.setItem(KEY, data);
      // SDK читает параметры из адреса, а грузится позже, когда адрес мог смениться: кладём их туда,
      // где он ищет сохранённые
      const params: Record<string, string> = {};
      hash.forEach((v, k) => (params[k] = v));
      sessionStorage.setItem("__telegram__initParams", JSON.stringify(params));
      // тема мессенджера до первой отрисовки: SDK ещё не загружен, а цвет фона уже в параметрах
      const bg = (JSON.parse(hash.get("tgWebAppThemeParams") ?? "{}") as { bg_color?: string }).bg_color ?? "";
      if (/^#[0-9a-f]{6}$/i.test(bg)) applyTheme(luma(bg) < 128 ? "dark" : "light");
    } else {
      initData = sessionStorage.getItem(KEY) ?? "";
    }
  } catch {
    // без хранилища: мини-приложение работает как обычный сайт
  }
  if (initData) document.documentElement.classList.add("in-tg");
}

export const inTelegram = () => initData !== "";
export const tgInitData = () => initData;

let loading: Promise<TgWebApp | null> | null = null;

// telegram — SDK мессенджера; вне Telegram сразу null.
export function telegram(): Promise<TgWebApp | null> {
  if (!inTelegram()) return Promise.resolve(null);
  if (!loading) {
    loading = new Promise((resolve) => {
      if (window.Telegram?.WebApp) return resolve(window.Telegram.WebApp);
      const s = document.createElement("script");
      s.src = SDK;
      s.async = true;
      s.onload = () => resolve(window.Telegram?.WebApp ?? null);
      s.onerror = () => resolve(null);
      document.head.appendChild(s);
    });
  }
  return loading;
}

// initTelegram — мини-приложение на весь экран, цвета шапки и фона Telegram под страницу,
// тема как у мессенджера.
export function initTelegram() {
  captureTelegram();
  void telegram().then((tg) => {
    if (!tg) return;
    tg.ready();
    tg.expand();
    const paint = () => {
      applyTheme(tg.colorScheme); // без записи в хранилище: на сайте у человека может быть своя тема
      const bg = hexColor(getComputedStyle(document.body).backgroundColor);
      if (!bg) return;
      tg.setHeaderColor(bg);
      tg.setBackgroundColor(bg);
      tg.setBottomBarColor?.(bg);
    };
    paint();
    tg.onEvent("themeChanged", paint);
  });
}

// haptic — лёгкий отклик под пальцем при отметке покупки; вне Telegram ничего.
export function haptic() {
  window.Telegram?.WebApp?.HapticFeedback?.selectionChanged();
}

// openInTelegram — ссылку t.me внутри мини-приложения открывает сам мессенджер, а не встроенный браузер.
export function openInTelegram(url: string): boolean {
  const tg = window.Telegram?.WebApp;
  if (!inTelegram() || !tg) return false;
  tg.openTelegramLink(url);
  return true;
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
