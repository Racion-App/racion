import type { Lang } from "../i18n";
import { decimalSep, intlLocale, pluralKey, tStatic, localeMeta } from "../i18n";
import type { Country } from "./types";

// Сумма в валюте страны плана: «1 234 ₽», «12,50 Br», «1 500 ₸», «12,50 €», «$12.50».
export function money(v: number, c: Country | undefined, lang?: Lang): string {
  if (!c) return `${Math.round(v).toLocaleString("ru-RU")} ₽`;
  try {
    // числа — в локали интерфейса, валюта — страны плана: «€110.79» по-английски, «110,79 €» по-немецки
    // narrowSymbol: «₸», «zł», «kr» вместо кодов «KZT», «PLN» в чужой локали
    return new Intl.NumberFormat(lang ? intlLocale(lang) : c.locale, { style: "currency", currency: c.currency, currencyDisplay: "narrowSymbol", minimumFractionDigits: c.decimals, maximumFractionDigits: c.decimals }).format(v);
  } catch {
    return `${v.toFixed(c.decimals)} ${c.symbol}`;
  }
}
export const approx = (v: number, c: Country | undefined, lang?: Lang) => `≈ ${money(v, c, lang)}`;

export function qty(v: number, unit: string, lang: Lang = "ru"): string {
  const u = (k: string) => tStatic(`unit.${k}`);
  // меньше грамма (перец, специи) — один знак после запятой, а не «0 г»
  const small = (n: number) => (n > 0 && n < 1 ? n.toFixed(1).replace(".", decimalSep(lang)) : String(Math.round(n)));
  if (unit === "pcs") return `${v > 0 && v < 1 ? trim(v, lang) : Math.round(v)} ${u("pcs")}`;
  if (unit === "ml") return v >= 1000 ? `${trim(v / 1000, lang)} ${u("l")}` : `${small(v)} ${u("ml")}`;
  return v >= 1000 ? `${trim(v / 1000, lang)} ${u("kg")}` : `${small(v)} ${u("g")}`;
}

function trim(v: number, lang: Lang) {
  const s = v.toFixed(2).replace(/\.?0+$/, "");
  return s.replace(".", decimalSep(lang));
}

export function minutes(m: number): string {
  if (m < 60) return `${m} ${tStatic("min")}`;
  const h = Math.floor(m / 60);
  const r = m % 60;
  return r ? tStatic("hoursMin", { h, m: r }) : tStatic("hours", { h });
}

function dayMonth(d: Date, lang: Lang): string {
  return d.toLocaleDateString(intlLocale(lang), { day: "numeric", month: "short" }).replace(/\.$/, "").replace(".", "");
}

// «сентябрь 2026» из "2026-09" по названиям месяцев языка (из _meta локали), запас — Intl
export function monthLabel(ym: string, lang: Lang = "ru"): string {
  const [y, m] = ym.split("-").map(Number);
  const names = localeMeta(lang)?.months;
  if (names && names.length === 12) return `${names[m - 1]} ${y}`;
  return new Date(y, m - 1, 1).toLocaleDateString(intlLocale(lang), { month: "long", year: "numeric" });
}

export function dateShort(iso: string, lang: Lang = "ru"): string {
  return dayMonth(new Date(iso + "T00:00:00"), lang);
}

export function weekRange(from: string, lang: Lang = "ru"): string {
  const a = new Date(from + "T00:00:00");
  const b = new Date(a);
  b.setDate(a.getDate() + 6);
  const same = a.getMonth() === b.getMonth();
  if (lang === "en") {
    // по-английски месяц идёт первым: «Sep 21–27», «Sep 28 – Oct 4»
    const m = (d: Date) => d.toLocaleDateString("en-US", { month: "short" });
    return same ? `${m(a)} ${a.getDate()}–${b.getDate()}` : `${dayMonth(a, lang)} – ${dayMonth(b, lang)}`;
  }
  const fa = same ? String(a.getDate()) : dayMonth(a, lang);
  return `${fa}–${dayMonth(b, lang)}`;
}

export function plural(n: number, one: string, few: string, many: string, lang: Lang = "ru"): string {
  const k = pluralKey(lang, n);
  return k === "one" ? one : k === "few" ? few : many;
}

export function people(adults: number, children: number, lang: Lang = "ru"): string {
  const w = (key: string, n: number) => tStatic(`${key}.${pluralKey(lang, n)}`);
  const parts = [`${adults} ${w("people.adults", adults)}`];
  if (children > 0) parts.push(`${children} ${w("people.kids", children)}`);
  return parts.join(", ");
}
