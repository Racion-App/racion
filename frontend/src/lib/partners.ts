import { api } from "./api";
import type { Partner, PartnerView } from "./types";
import { readJSON, writeJSON } from "./storage";

// Партнёрские магазины страны: кэш на сессию, «скрыть» на 30 дней в localStorage.
// Ссылка «где купить» строится из шаблона первого магазина и слова для поиска на языке страны.

const HIDE_KEY = "racion.partners.hide";
const HIDE_DAYS = 30;
const cache = new Map<string, Promise<PartnerView>>();

export function partnersFor(country: string): Promise<PartnerView> {
  let p = cache.get(country);
  if (!p) {
    p = api.partners(country).catch(() => ({ goods: [], grocery: [], queries: {} }) as PartnerView);
    cache.set(country, p);
  }
  return p;
}

export function partnersHidden(): boolean {
  const until = readJSON<number>(HIDE_KEY, 0);
  return until > Date.now();
}

export function hidePartners(): void {
  writeJSON(HIDE_KEY, Date.now() + HIDE_DAYS * 86400_000);
}

export type BuyLink = { key: string; url: string; shop: Partner };

// Ссылки на технику из списка рецепта: только то, что реально покупают (ключи есть в queries).
export function buyLinks(view: PartnerView, equipment: string[]): BuyLink[] {
  const shop = view.goods[0];
  if (!shop) return [];
  return equipment.filter((e) => view.queries[e]).map((e) => ({ key: e, url: expand(shop.url, view.queries[e]), shop }));
}

export function expand(tpl: string, q: string): string {
  return tpl.replace("{q}", encodeURIComponent(q));
}
