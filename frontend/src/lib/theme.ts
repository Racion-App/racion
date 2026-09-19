// Тема: auto (системная) | light | dark. Хранится в localStorage, применяется через data-theme на <html>,
// который понимает дизайн-система. Стартовое применение — инлайн-скриптом в index.html, чтобы не мигало.
export type Theme = "auto" | "light" | "dark";
const KEY = "racion.theme";

export function getTheme(): Theme {
  try {
    const v = localStorage.getItem(KEY);
    return v === "light" || v === "dark" ? v : "auto";
  } catch {
    return "auto";
  }
}

export function applyTheme(t: Theme) {
  const root = document.documentElement;
  if (t === "auto") root.removeAttribute("data-theme");
  else root.setAttribute("data-theme", t);
}

export function setTheme(t: Theme) {
  try {
    if (t === "auto") localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, t);
  } catch {
    // без хранилища — тема живёт до перезагрузки
  }
  applyTheme(t);
}

export const THEME_LABEL: Record<Theme, string> = { auto: "Как в системе", light: "Светлая", dark: "Тёмная" };
export const THEME_NEXT: Record<Theme, Theme> = { auto: "light", light: "dark", dark: "auto" };
