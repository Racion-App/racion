import type { Params } from "./types";
import { readJSON, writeJSON } from "./storage";

export const DRAFT_KEY = "racion.quiz.v4";

// Пресет «Пост» раньше записывался в черновик навсегда и потом резал праздничные столы до «нарезки».
// Черновик с его точной подписью (без мяса, птицы, рыбы, морепродуктов, субпродуктов + аллергия
// на молоко и яйца + цель «здоровье») чистим при чтении; настоящие ограничения так не выглядят.
const LENT_TAGS = ["meat", "poultry", "fish", "seafood", "offal"];

export function isLentPreset(d: Partial<Params>): boolean {
  const tags = [...(d.excludeTags ?? [])].sort().join(",");
  const al = d.allergens ?? [];
  return tags === [...LENT_TAGS].sort().join(",") && al.includes("dairy") && al.includes("eggs") && d.goal === "healthy";
}

export function readDraft(): Partial<Params> {
  const d = readJSON<Partial<Params>>(DRAFT_KEY, {});
  if (isLentPreset(d)) {
    const clean = { ...d, excludeTags: [], allergens: [], goal: "none", members: d.members?.map((m) => ({ ...m, goal: "none" })) };
    writeJSON(DRAFT_KEY, clean);
    return clean;
  }
  return d;
}

// Снять все ограничения в черновике: стоп-продукты, теги, аллергены.
export function clearDraftLimits(): Partial<Params> {
  const d = readJSON<Partial<Params>>(DRAFT_KEY, {});
  const clean = { ...d, excludeTags: [], allergens: [], exclude: [] };
  writeJSON(DRAFT_KEY, clean);
  return clean;
}
