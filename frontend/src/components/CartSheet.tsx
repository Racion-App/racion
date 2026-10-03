import { useEffect, useMemo, useState } from "react";
import { Copy, ExternalLink, ShoppingCart } from "lucide-react";
import { Sheet } from "./Sheet";
import { cartTargets, searchURL, type CartTarget } from "../lib/cart";
import { track } from "../lib/analytics";
import { api } from "../lib/api";
import type { StoreCart } from "../lib/types";
import { partnersFor } from "../lib/partners";
import { OfferCard, useOffer } from "./OfferCard";
import { useT } from "../i18n";

// «Собрать корзину»: выбор магазина или доставки, список ещё не купленного со ссылками на поиск
// и «скопировать список» для приложений без поиска по ссылке. Если у сети есть свой каталог
// (ВкусВилл), сверху корзина одной ссылкой: самое дорогое из списка, до 20 позиций.

type Item = { id: string; name: string; qty: string };

export function CartSheet({ open, onClose, planId, storeCart, storeCode, storeName, country, region, items, onToast }: { open: boolean; onClose: () => void; planId: string; storeCart: boolean; storeCode: string; storeName: string; country: string; region?: string; items: Item[]; onToast: (s: string) => void }) {
  const { t } = useT();
  // корзина сети ссылкой: только продукты из списка, свои позиции («молоко для кофе») сеть не знает
  const ids = useMemo(() => items.filter((i) => !i.id.startsWith("extra:")).map((i) => i.id), [items]);
  const [made, setMade] = useState<StoreCart | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const idsKey = ids.join(",");
  useEffect(() => {
    // отметили что-то купленным или закрыли лист — старая ссылка уже не про этот список
    setMade(null);
    setErr(null);
  }, [open, idsKey]);
  const build = async () => {
    setBusy(true);
    setErr(null);
    try {
      const r = await api.storeCart(planId, ids);
      setMade(r);
      track("store_cart", { store: storeCode, added: r.added, wanted: r.wanted });
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const offer = useOffer("cart", open ? country : undefined, region);
  const [extra, setExtra] = useState<CartTarget[]>([]);
  useEffect(() => {
    if (!open) return;
    let alive = true;
    // доставки из админки (с партнёрскими параметрами) идут перед встроенными; совпадения по коду не дублируем
    partnersFor(country).then((v) => alive && setExtra(v.grocery.map((g) => ({ code: g.code, name: g.name, url: g.url, affiliate: g.affiliate }))));
    return () => {
      alive = false;
    };
  }, [open, country]);
  const targets = useMemo(() => {
    const base = cartTargets(storeCode, storeName, country);
    const seen = new Set(extra.map((x) => x.name.toLowerCase()));
    return [...base.filter((b) => b.code === storeCode), ...extra, ...base.filter((b) => b.code !== storeCode && !seen.has(b.name.toLowerCase()))];
  }, [storeCode, storeName, country, extra]);
  const [picked, setPicked] = useState<string | null>(null);
  const target = targets.find((x) => x.code === picked) ?? targets[0] ?? null;
  const setTarget = (x: CartTarget) => setPicked(x.code);
  const copy = async () => {
    const text = items.map((i) => `${i.name} — ${i.qty}`).join("\n");
    try {
      await navigator.clipboard.writeText(text);
      onToast(t("cart.copied"));
      track("cart_copy");
    } catch {
      onToast(t("cart.copy.fail"));
    }
  };
  return (
    <Sheet open={open} onClose={onClose} title={t("cart.title")} closeLabel={t("close")}>
      <h2>{t("cart.title")}</h2>
      <p className="bsheet__lead">{t("cart.lead")}</p>
      {storeCart && (
        <section className="cart__store" aria-label={t("cart.store.title", { store: storeName })}>
          <h3>{t("cart.store.title", { store: storeName })}</h3>
          <p className="cart__store-lead">{t("cart.store.lead")}</p>
          {made ? (
            <>
              <a className="btn btn-primary" href={made.link} target="_blank" rel="noopener" onClick={() => track("store_cart_open", { store: storeCode, added: made.added })}>
                {t("cart.store.open")} <ExternalLink size={16} aria-hidden />
              </a>
              <p className="cart__store-note">
                {t("cart.store.done", { added: made.added, wanted: made.wanted })}
                {made.left.length > 0 && <> · {t("cart.store.left")}</>}
              </p>
            </>
          ) : (
            <button type="button" className="btn btn-primary" disabled={busy || ids.length === 0} onClick={build} aria-busy={busy}>
              <ShoppingCart size={16} aria-hidden /> {busy ? t("cart.store.busy") : t("cart.store.button")}
            </button>
          )}
          {err && <p className="error-inline" role="alert">{err}</p>}
        </section>
      )}
      {targets.length > 0 && (
        <div className="segmented cart__targets" role="radiogroup" aria-label={t("cart.where")}>
          {targets.map((x) => (
            <button key={x.code} type="button" role="radio" aria-checked={target?.code === x.code} onClick={() => setTarget(x)}>
              {x.name}
            </button>
          ))}
        </div>
      )}
      {target?.affiliate && <p className="partner__note">{t("partner.ad")}</p>}
      {offer && <OfferCard offer={offer} compact />}
      <ul className="cart__list">
        {items.map((i) => (
          <li key={i.id} className="cart__item">
            <span className="cart__name">
              {i.name} <small className="num">{i.qty}</small>
            </span>
            {target && (
              <a className="btn btn-soft btn-sm" href={searchURL(target, i.name)} target="_blank" rel={target.affiliate ? "sponsored nofollow noopener" : "noopener"} onClick={() => track("cart_open", { store: target.code })}>
                {t("cart.find")} <ExternalLink size={14} aria-hidden />
              </a>
            )}
          </li>
        ))}
      </ul>
      {items.length === 0 && <p className="ownform__empty">{t("cart.empty")}</p>}
      <div className="cart__actions">
        <button type="button" className={storeCart ? "btn btn-soft" : "btn btn-primary"} onClick={copy} disabled={items.length === 0}>
          <Copy size={16} aria-hidden /> {t("cart.copy")}
        </button>
        <span className="cart__hint">
          <ShoppingCart size={14} aria-hidden /> {t("cart.hint")}
        </span>
      </div>
    </Sheet>
  );
}
