import type { Country, Plan, Recipe, RecipePage } from "./api.js";
import { BASE } from "./api.js";

// Модель читает текст, а не JSON: каждый ответ — короткая сводка, по которой можно сразу говорить с человеком.
// Полные данные идут структурой рядом, чтобы клиент мог ими воспользоваться.

export function money(v: number, c: Country | undefined): string {
  if (!c) return String(Math.round(v));
  try {
    return new Intl.NumberFormat("en-US", { style: "currency", currency: c.currency, currencyDisplay: "narrowSymbol", maximumFractionDigits: c.decimals }).format(v);
  } catch {
    return `${v.toFixed(c.decimals)} ${c.symbol}`;
  }
}

export function recipesText(p: RecipePage, c: Country | undefined): string {
  if (p.total === 0) return "Nothing matched. Try a shorter query or drop the meal filter.";
  const rows = p.items.map((r) => `- ${r.Title} — ${r.TimeMin} min, ${Math.round(r.Kcal)} kcal, about ${money(r.Cost, c)} a portion. id: ${r.ID}`);
  const shown = p.offset + p.items.length;
  const tail = shown < p.total ? `\n\nShowing ${p.offset + 1}–${shown} of ${p.total}. Ask for the next page with offset ${shown}.` : "";
  return `${p.total} recipes found.\n\n${rows.join("\n")}${tail}`;
}

export function recipeText(r: Recipe, c: Country | undefined): string {
  const ing = r.ingredients
    .map((i) => `- ${i.name}: ${round(i.amount)} ${i.unit}${i.pantry ? " (usually at home)" : ""}`)
    .join("\n");
  const steps = r.steps.map((s, i) => `${i + 1}. ${s}`).join("\n");
  return [
    `# ${r.title}`,
    r.description,
    "",
    `${r.timeMin} min · ${Math.round(r.kcal)} kcal · protein ${Math.round(r.protein)} g · fat ${Math.round(r.fat)} g · carbs ${Math.round(r.carb)} g`,
    `About ${money(r.cost, c)} a portion — an estimate from national statistics, not a shelf price.`,
    "",
    "## Ingredients for one portion",
    ing,
    "",
    "## Steps",
    steps,
    "",
    `Page for a person: ${BASE}/recipe/${r.id}`,
  ].join("\n");
}

export function planText(p: Plan, title: string): string {
  const c = p.country;
  const days = p.days
    .map((d) => {
      const dishes = d.dishes
        .map((x) => `  ${x.slot}: ${x.title} — ${x.timeMin} min, ${Math.round(x.kcal)} kcal, ${money(x.cost, c)}${x.leftover ? " (yesterday's)" : ""}${x.servings ? ` · for ${x.servings}` : ""}`)
        .join("\n");
      return `${d.label} ${d.date} — ${Math.round(d.kcal)} kcal, ${money(d.cost, c)}\n${dishes}`;
    })
    .join("\n\n");
  const list = p.shopping
    .map((g) => {
      const items = g.items
        .map((i) => `  ${i.name}: ${round(i.buy)} ${i.unit}${i.needed < i.buy ? ` (recipes need ${round(i.needed)})` : ""}${i.pantry ? " — usually at home" : ` — ${money(i.cost, c)}`}`)
        .join("\n");
      return `${g.label ?? g.category}${g.cost ? ` — ${money(g.cost, c)}` : ""}\n${items}`;
    })
    .join("\n\n");
  const leftover = p.totals.usedCost !== undefined ? `\nEaten during the plan: ${money(p.totals.usedCost, c)}. The difference stays as leftover packs for later.` : "";
  const warn = p.warnings?.length ? `\n\nWarnings: ${p.warnings.join("; ")}` : "";
  return [
    `# ${title}`,
    `${p.store?.name ?? ""}${p.store?.name ? " · " : ""}${Math.round(p.totals.kcalPerDay)} kcal a day · ${p.totals.items} items on the list`,
    `Groceries: about ${money(p.totals.cost, c)}.${leftover}`,
    p.priceSource ? `Prices: ${[p.priceSource.name, p.priceSource.region, p.priceSource.period].filter(Boolean).join(", ")} — an estimate, not a shelf price.` : "",
    "",
    days,
    "",
    "## Shopping list",
    "Amounts are what to buy, rounded up to whole packs.",
    "",
    list,
    warn,
    "",
    `Open the plan: ${BASE}/plan/${p.id} — send this link to the person instead of retelling the list.`,
  ]
    .filter(Boolean)
    .join("\n");
}

function round(v: number): number {
  return v < 10 ? Math.round(v * 10) / 10 : Math.round(v);
}
