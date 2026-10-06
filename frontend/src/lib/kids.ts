import type { Child } from "./types";
import { DRAFT_KEY, readDraft } from "./draft";
import { writeJSON } from "./storage";
// Возраст и кормление детей: общие помощники квиза и раздела «Семья».
import { pluralKey, type Lang } from "../i18n";

type T = (k: string, p?: Record<string, string | number>) => string;

// ageLabel — возраст как говорят родители: до года — «8 мес», до трёх лет — «1 год 3 мес», дальше — «5 лет».
// Так же подписывает бэкенд (planner.Child.AgeLabel).
export function ageLabel(t: T, lang: Lang, m: number): string {
  if (m === 0) return t("age.newborn");
  if (m < 12) return t("age.months", { n: m });
  const y = Math.floor(m / 12);
  const years = `${y} ${t(`age.years.${pluralKey(lang, y)}`)}`;
  return m < 36 && m % 12 ? t("age.ym", { y: years, m: m % 12 }) : years;
}

// Возраст ребёнка в списке: до трёх лет — по месяцам (программы питания делят возраст на 1–1,5 и 1,5–3 года),
// дальше — по годам. Возраст растёт сам, поэтому текущее значение ребёнка добавляем, если его нет в сетке.
export function ageOptions(t: T, lang: Lang, current?: number): { months: number; label: string }[] {
  const months = [...Array.from({ length: 36 }, (_, m) => m), ...Array.from({ length: 15 }, (_, i) => (i + 3) * 12)];
  if (current !== undefined && current >= 0 && !months.includes(current)) months.push(current);
  return months.sort((a, b) => a - b).map((m) => ({ months: m, label: ageLabel(t, lang, m) }));
}

// Режимы кормления по возрасту, как в backend/internal/planner/kids.go (первый — по умолчанию).
// Прикорм с 4–6 мес по решению педиатра, поэтому в 4–5 мес он есть, но не по умолчанию.
export function feedingOptions(m: number): string[] {
  if (m < 4) return ["milk"];
  if (m < 6) return ["milk", "weaning"];
  if (m < 12) return ["weaning", "jars", "separate", "shared", "mix"];
  if (m < 36) return ["shared", "separate", "jars", "mix"];
  return ["shared", "separate", "mix"];
}

// Части суток, когда ребёнок получает смесь, и объёмы — как в backend/internal/planner/kids.go.
export const FORMULA_SLOTS = ["morning", "day", "bedtime", "night"] as const;

function formulaPerFeed(m: number): number {
  if (m < 1) return 80;
  if (m < 2) return 110;
  if (m < 4) return 140;
  if (m < 6) return 180;
  return 200;
}

function formulaFeedsIn(slot: string, m: number): number {
  if (slot === "morning" || slot === "bedtime") return 1;
  if (slot === "night") return m < 2 ? 2 : 1;
  if (slot === "day") return m < 1 ? 4 : m < 4 ? 3 : m < 6 ? 2 : m < 8 ? 1 : 0;
  return 0;
}

// formulaMlByAge — смесь в сутки: по отмеченным кормлениям или, если не отмечено, по возрастной норме.
export function formulaMlByAge(m: number, feeds?: string[]): number {
  if (feeds && feeds.length > 0) return feeds.reduce((n, f) => n + formulaFeedsIn(f, m), 0) * formulaPerFeed(m);
  if (m < 1) return 600;
  if (m < 2) return 750;
  if (m < 4) return 850;
  if (m < 6) return 900;
  if (m < 9) return 700;
  if (m < 12) return 500;
  if (m < 24) return 350;
  if (m < 36) return 250;
  return 200;
}

// Режим «комбинирую»: приёмы и источники, как planner.MealSlots и Child.normalized.
export function mealSlots(m: number): string[] {
  return m < 12 ? ["breakfast", "lunch", "dinner"] : ["breakfast", "lunch", "dinner", "snack"];
}

export function mealSourceOptions(m: number): string[] {
  return m < 36 ? ["home", "jars", "shared"] : ["home", "shared"];
}

// ymNow — текущий месяц в виде YYYY-MM: отметка, когда указан возраст ребёнка.
export function ymNow(d = new Date()): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

// grownKid — возраст вырос с отметки ageAt: «8 мес» в октябре — в декабре уже 10. Без отметки — ставим
// текущий месяц, дальше возраст растёт сам. Режим кормления меняется, если по новому возрасту прежний
// недоступен (в год прикорм по месяцам заканчивается). То же делает бэкенд к дате начала плана.
// kidId — постоянный номер ребёнка (как на сервере, service.kidIDs): по нему дневник находит ребёнка в семье.
export function kidId(): string {
  const b = new Uint8Array(6);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

export function grownKid(k: Child, now = new Date()): Child {
  const cur = ymNow(now);
  if (!k.id) k = { ...k, id: kidId() };
  if (!k.ageAt || !/^\d{4}-\d{2}$/.test(k.ageAt)) return { ...k, ageAt: cur };
  const [y, m] = k.ageAt.split("-").map(Number);
  const diff = (now.getFullYear() - y) * 12 + (now.getMonth() + 1 - m);
  if (diff <= 0) return k;
  const ageMonths = Math.min(k.ageMonths + diff, 17 * 12);
  const opts = feedingOptions(ageMonths);
  return { ...k, ageMonths, ageAt: cur, feeding: opts.includes(k.feeding) ? k.feeding : opts[0] };
}

export const grownKids = (kids: Child[] | undefined): Child[] => (kids ?? []).map((k) => grownKid(k));

// Дневник прикорма. Неделю строят из черновика квиза, поэтому отметки «ввели» и «была реакция» пишем
// туда: следующая неделя сама предложит новый продукт или обойдёт неподошедший. Кнопки показываем,
// только если в черновике тот же ребёнок (номер, режим, возраст ±1 мес): у гостя по чужой ссылке
// черновика с этим ребёнком нет.
export function draftKid(planKid: Child | undefined, idx: number): Child | null {
  if (!planKid) return null;
  const k = readDraft().kids?.[idx];
  if (!k || k.feeding !== planKid.feeding || Math.abs(k.ageMonths - planKid.ageMonths) > 1) return null;
  return k;
}

// markWeaning — отметка в черновике; возвращает id ребёнка, чтобы вошедший записал её и в аккаунт.
export function markWeaning(idx: number, food: string, how: "ok" | "reaction"): string | undefined {
  const d = readDraft();
  const today = new Date().toISOString().slice(0, 10);
  let id: string | undefined;
  const kids = (d.kids ?? []).map((k, i) => {
    if (i !== idx) return k;
    id = k.id;
    return markKid(k, food, how, today);
  });
  writeJSON(DRAFT_KEY, { ...d, kids });
  return id;
}

// markKid — то же, что planner.Child.MarkDiary: «ввели» — в «уже ест», «была реакция» — в «не подошло»,
// пустая отметка снимает; запись дня — в начало дневника.
export function markKid(k: Child, food: string, how: "ok" | "reaction" | "", date: string): Child {
  const introduced = (k.introduced ?? []).filter((x) => x !== food);
  const avoid = (k.avoid ?? []).filter((x) => x !== food);
  const diary = (k.diary ?? []).filter((e) => e.food !== food);
  if (how === "ok") return { ...k, introduced: [...introduced, food], avoid, diary: [{ food, how, date }, ...diary].slice(0, 60) };
  if (how === "reaction") return { ...k, introduced, avoid: [...avoid, food], diary: [{ food, how, date }, ...diary].slice(0, 60) };
  return { ...k, introduced, avoid, diary };
}

