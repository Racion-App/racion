// «Собрать корзину»: ссылки на поиск в интернет-магазине сети и в сервисах доставки страны.
// Без партнёрского API это deep-link с поиском по позиции: сайт открывается сразу на нужном товаре.
// {q} — название продукта. Если у сети нет шаблона, остаются сервисы доставки страны и «скопировать список».

export type CartTarget = { code: string; name: string; url: string };

const STORE_SEARCH: Record<string, string> = {
  pyaterochka: "https://5ka.ru/search/?text={q}",
  perekrestok: "https://www.perekrestok.ru/cat/search?search={q}",
  lenta: "https://lenta.com/search/?searchText={q}",
  vkusvill: "https://vkusvill.ru/search/?q={q}",
  metro: "https://online.metro-cc.ru/search?q={q}",
  dixy: "https://dixy.ru/catalog/?q={q}",
  magnit: "https://magnit.ru/search/?term={q}",
  auchan: "https://www.auchan.ru/search/?query={q}",
  evroopt: "https://edostavka.by/search?query={q}",
  walmart: "https://www.walmart.com/search?q={q}",
  target: "https://www.target.com/s?searchTerm={q}",
  kroger: "https://www.kroger.com/search?query={q}",
  costco: "https://www.costco.com/CatalogSearch?keyword={q}",
  wholefoods: "https://www.wholefoodsmarket.com/search?text={q}",
  safeway: "https://www.safeway.com/shop/search-results.html?q={q}",
  tesco: "https://www.tesco.com/groceries/en-GB/search?query={q}",
  sainsburys: "https://www.sainsburys.co.uk/gol-ui/SearchResults/{q}",
  asda: "https://groceries.asda.com/search/{q}",
  morrisons: "https://groceries.morrisons.com/search?entry={q}",
  waitrose: "https://www.waitrose.com/ecom/shop/search?searchTerm={q}",
  rewe: "https://shop.rewe.de/productList?search={q}",
  kaufland: "https://www.kaufland.de/s/?search_value={q}",
  lidl: "https://www.lidl.de/q/search?q={q}",
  carrefour: "https://www.carrefour.fr/s?q={q}",
  auchan_fr: "https://www.auchan.fr/recherche?text={q}",
  intermarche: "https://www.intermarche.com/recherche/{q}",
  mercadona: "https://tienda.mercadona.es/search-results?query={q}",
  dia: "https://www.dia.es/search?q={q}",
  albertheijn: "https://www.ah.nl/zoeken?query={q}",
  jumbo: "https://www.jumbo.com/producten/?searchType=keyword&searchTerms={q}",
};

const DELIVERY: Record<string, CartTarget[]> = {
  RU: [
    { code: "kuper", name: "Купер", url: "https://kuper.ru/search?q={q}" },
    { code: "lavka", name: "Яндекс Лавка", url: "https://lavka.yandex.ru/search?text={q}" },
    { code: "samokat", name: "Самокат", url: "https://samokat.ru/search?query={q}" },
  ],
  BY: [{ code: "edostavka", name: "Е-доставка", url: "https://edostavka.by/search?query={q}" }],
  KZ: [{ code: "arbuz", name: "Arbuz", url: "https://arbuz.kz/ru/almaty/search?q={q}" }],
  US: [{ code: "instacart", name: "Instacart", url: "https://www.instacart.com/store/s?k={q}" }],
  GB: [{ code: "ocado", name: "Ocado", url: "https://www.ocado.com/search?entry={q}" }],
  DE: [{ code: "flink", name: "Flink", url: "https://www.goflink.com/de-DE/search?q={q}" }],
  FR: [{ code: "carrefour", name: "Carrefour", url: "https://www.carrefour.fr/s?q={q}" }],
  ES: [{ code: "mercadona", name: "Mercadona", url: "https://tienda.mercadona.es/search-results?query={q}" }],
  NL: [{ code: "picnic", name: "Albert Heijn", url: "https://www.ah.nl/zoeken?query={q}" }],
};

// Куда можно собрать корзину: сначала своя сеть (если у неё есть поиск), потом доставка страны.
export function cartTargets(storeCode: string, storeName: string, country: string): CartTarget[] {
  const out: CartTarget[] = [];
  if (STORE_SEARCH[storeCode]) out.push({ code: storeCode, name: storeName, url: STORE_SEARCH[storeCode] });
  for (const d of DELIVERY[country] ?? []) if (d.code !== storeCode) out.push(d);
  return out;
}

export function searchURL(target: CartTarget, name: string): string {
  // ищем по первому слову-двум: «Куриное филе» найдётся, «Куриное филе охлаждённое 500 г» — нет
  const q = name.split(/[,(]/)[0].trim().split(/\s+/).slice(0, 2).join(" ");
  return target.url.replace("{q}", encodeURIComponent(q));
}
