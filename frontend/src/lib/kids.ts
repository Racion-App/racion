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

export function feedingOptions(m: number): string[] {
  if (m < 6) return ["milk"];
  if (m < 12) return ["jars", "separate", "shared"];
  if (m < 36) return ["shared", "separate", "jars"];
  return ["shared", "separate"];
}

export function formulaMlByAge(m: number): number {
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
