import { readJSON, writeJSON } from "./storage";
import type { Plan, PlanSummary } from "./types";

// Недели, открытые в этом браузере. Без аккаунта сервер не знает, чья неделя, и главная предлагала
// анкету с нуля: закрыл вкладку, не сохранив адрес, — неделя потеряна. Теперь браузер помнит недели сам:
// главная показывает последнюю, установленное приложение открывает ту, что идёт сейчас.
// Только недели: праздничные столы живут своими страницами.

type LocalWeek = { id: string; start: string; items: number; store: string; seen: number };

const KEY = "racion.weeks";
const MAX = 10;

function iso(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function addDays(day: string, n: number): string {
  const d = new Date(day + "T00:00:00");
  d.setDate(d.getDate() + n);
  return iso(d);
}

function read(): LocalWeek[] {
  const list = readJSON<LocalWeek[]>(KEY, []);
  return Array.isArray(list) ? list.filter((w) => w && typeof w.id === "string" && typeof w.start === "string") : [];
}

// rememberWeek — неделю открыли или собрали: она первая в списке.
export function rememberWeek(p: Plan) {
  if (p.occasion || p.basket || !p.days.length) return;
  const w: LocalWeek = { id: p.id, start: p.days[0].date, items: p.totals.items, store: p.store.name, seen: Date.now() };
  writeJSON(KEY, [w, ...read().filter((x) => x.id !== p.id)].slice(0, MAX));
}

// forgetWeek — недели больше нет на сервере: не предлагаем открыть пустую ссылку.
export function forgetWeek(id: string) {
  writeJSON(KEY, read().filter((x) => x.id !== id));
}

// currentWeek — последняя открытая неделя, которая идёт сегодня.
export function currentWeek(today = iso(new Date())): LocalWeek | undefined {
  return read().find((w) => w.start <= today && today <= addDays(w.start, 6));
}

// lastWeekSummary — последняя открытая неделя в виде строки списка недель, как у аккаунта;
// куплено — по отметкам, которые браузер хранит у себя.
export function lastWeekSummary(): PlanSummary | null {
  const w = currentWeek() ?? read()[0];
  if (!w) return null;
  const checked = Object.values(readJSON<Record<string, boolean>>(`racion.check.${w.id}`, {})).filter(Boolean).length;
  return { id: w.id, title: "", startDate: w.start, store: w.store, cost: 0, portions: 0, createdAt: new Date(w.seen).toISOString(), checked, items: w.items };
}

// nextWeekStart — с какого понедельника собирать следующую неделю: сразу после последней, если она
// ещё не кончилась, иначе с ближайшего понедельника (сегодня, если сегодня понедельник).
export function nextWeekStart(lastStart: string | undefined, now = new Date()): string {
  const today = iso(now);
  if (lastStart && addDays(lastStart, 6) >= today) return addDays(lastStart, 7);
  const d = new Date(now);
  d.setHours(0, 0, 0, 0);
  const wd = d.getDay();
  d.setDate(d.getDate() + (wd === 1 ? 0 : (8 - wd) % 7));
  return iso(d);
}

// openCurrentWeek — установленное приложение стартует с главной (/?source=pwa). Если в браузере есть
// неделя, которая идёт сейчас, открываем сразу её: с приложением ходят в магазин и смотрят, что готовить
// сегодня. Вызывается до роутера; true — адрес подменён.
export function openCurrentWeek(): boolean {
  try {
    if (location.pathname !== "/" || new URLSearchParams(location.search).get("source") !== "pwa") return false;
    const w = currentWeek();
    if (!w) return false;
    history.replaceState(null, "", `/plan/${w.id}`);
    return true;
  } catch {
    return false;
  }
}
