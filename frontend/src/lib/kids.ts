import type { Child } from "./types";
import { DRAFT_KEY, readDraft } from "./draft";
import { writeJSON } from "./storage";
// Возраст и кормление детей: общие помощники квиза и раздела «Семья».
import { pluralKey, type Lang } from "../i18n";

// Возраст ребёнка: до 2 лет — по месяцам, дальше — по годам.
export function ageOptions(t: (k: string, p?: Record<string, string | number>) => string, lang: Lang): { months: number; label: string }[] {
  return [
    ...Array.from({ length: 24 }, (_, m) => ({ months: m, label: t("age.months", { n: m }) })),
    ...Array.from({ length: 16 }, (_, i) => {
      const y = i + 2;
      return { months: y * 12, label: `${y} ${t(`age.years.${pluralKey(lang, y)}`)}` };
    }),
  ];
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
export function grownKid(k: Child, now = new Date()): Child {
  const cur = ymNow(now);
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

export function markWeaning(idx: number, food: string, how: "ok" | "reaction") {
  const d = readDraft();
  const kids = (d.kids ?? []).map((k, i) => {
    if (i !== idx) return k;
    const introduced = (k.introduced ?? []).filter((x) => x !== food);
    const avoid = (k.avoid ?? []).filter((x) => x !== food);
    return how === "ok" ? { ...k, introduced: [...introduced, food], avoid } : { ...k, introduced, avoid: [...avoid, food] };
  });
  writeJSON(DRAFT_KEY, { ...d, kids });
}
