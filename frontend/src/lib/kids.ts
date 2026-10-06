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
