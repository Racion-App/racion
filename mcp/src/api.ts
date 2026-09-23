// Тонкая обёртка над публичным API racion.app: ключей нет, только таймаут и понятные ошибки.
// Базу можно переопределить переменной RACION_API — это нужно на своём сервере и в тестах.

export const BASE = (process.env.RACION_API || "https://racion.app").replace(/\/+$/, "");
const TIMEOUT_MS = Number(process.env.RACION_TIMEOUT_MS || 30000);
const UA = "racion-mcp/1.0 (+https://racion.app)";

export class ApiError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
    this.name = "ApiError";
  }
}

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), TIMEOUT_MS);
  let res: Response;
  try {
    res = await fetch(BASE + path, {
      ...init,
      signal: ctrl.signal,
      headers: { "User-Agent": UA, Accept: "application/json", ...(init?.body ? { "Content-Type": "application/json" } : {}), ...init?.headers },
    });
  } catch (e) {
    clearTimeout(timer);
    if ((e as Error).name === "AbortError") throw new ApiError(0, `racion.app did not answer within ${TIMEOUT_MS} ms`);
    throw new ApiError(0, `cannot reach racion.app: ${(e as Error).message}`);
  }
  clearTimeout(timer);
  if (!res.ok) {
    // ответ об ошибке — маленький JSON вида {"error": "..."} либо пустое тело
    let detail = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body?.error) detail = body.error;
    } catch {
      /* тело не JSON — оставляем статус */
    }
    if (res.status === 429) throw new ApiError(429, "rate limit reached on racion.app, try again in a minute");
    throw new ApiError(res.status, detail);
  }
  return (await res.json()) as T;
}

export const api = {
  meta: (country?: string, lang?: string) => call<Meta>(`/api/meta${qs({ country, lang })}`),
  recipes: (p: { q?: string; slot?: string; country?: string; lang?: string; limit?: number; offset?: number }) =>
    call<RecipePage>(`/api/recipes${qs(p)}`),
  recipe: (id: string, country?: string, lang?: string) =>
    call<Recipe>(`/api/recipes/${encodeURIComponent(id)}${qs({ country, lang })}`),
  collections: (lang?: string) => call<Collection[]>(`/api/collections${qs({ lang })}`),
  occasions: (lang?: string) => call<Occasion[]>(`/api/occasions${qs({ lang })}`),
  plan: (params: unknown) => call<Plan>("/api/plans", { method: "POST", body: JSON.stringify(params) }),
  basket: (body: unknown) => call<Plan>("/api/baskets", { method: "POST", body: JSON.stringify(body) }),
  occasionPlan: (id: string, body: unknown) =>
    call<Plan>(`/api/occasions/${encodeURIComponent(id)}`, { method: "POST", body: JSON.stringify(body) }),
};

function qs(p: Record<string, string | number | undefined>): string {
  const s = Object.entries(p)
    .filter(([, v]) => v !== undefined && v !== "")
    .map(([k, v]) => `${k}=${encodeURIComponent(String(v))}`)
    .join("&");
  return s ? `?${s}` : "";
}

export type Country = { code: string; label?: string; currency: string; symbol: string; decimals: number; hasRegions?: boolean };
export type Store = { code: string; country: string; name: string; kind: string; priceIndex: number; note?: string };
export type Meta = {
  countries: Country[];
  stores: Store[];
  allergens?: { id: string; label: string }[];
  excludePresets?: { id: string; kind: string; label: string }[];
};
export type RecipeCard = { ID: string; Title: string; Slot: string; TimeMin: number; Kcal: number; Cost: number; Href: string; Tags?: string[] };
export type RecipePage = { total: number; offset: number; limit: number; items: RecipeCard[] };
export type Recipe = {
  id: string;
  title: string;
  description: string;
  slot: string;
  timeMin: number;
  tags: string[];
  equipment: string[];
  steps: string[];
  ingredients: { ingredientId: string; name: string; amount: number; unit: string; pantry: boolean }[];
  kcal: number;
  protein: number;
  fat: number;
  carb: number;
  cost: number;
};
export type Collection = { slug: string; name: string; description?: string; recipes?: string[] };
export type Occasion = { id: string; title: string; lead?: string; guests?: number; kind?: string };
export type Dish = { slot: string; recipeId: string; title: string; timeMin: number; kcal: number; cost: number; leftover?: boolean; batch?: boolean; servings?: number; why?: string };
export type Day = { index: number; date: string; label: string; kcal: number; cost: number; dishes: Dish[] };
export type ShopItem = { name: string; needed: number; buy: number; unit: string; cost: number; pantry?: boolean };
export type ShopGroup = { category: string; label?: string; cost: number; items: ShopItem[] };
export type PriceSource = { name: string; period?: string; region?: string; coverage?: number };
export type Plan = {
  id: string;
  country: Country;
  store: Store;
  days: Day[];
  shopping: ShopGroup[];
  totals: { cost: number; usedCost?: number; items: number; kcalPerDay: number; proteinPerDay?: number; cookMin?: number };
  priceSource?: PriceSource;
  warnings?: string[];
};
