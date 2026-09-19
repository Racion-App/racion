import { useEffect, useMemo, useState } from "react";
import { Copy, ExternalLink, ShoppingCart } from "lucide-react";
import { Sheet } from "./Sheet";
import { cartTargets, searchURL, type CartTarget } from "../lib/cart";
import { track } from "../lib/analytics";
import { partnersFor } from "../lib/partners";
import { useT } from "../i18n";

// «Собрать корзину»: выбор магазина или доставки, список ещё не купленного со ссылками на поиск
// и «скопировать список» для приложений без поиска по ссылке.

type Item = { id: string; name: string; qty: string };

export function CartSheet({ open, onClose, storeCode, storeName, country, items, onToast }: { open: boolean; onClose: () => void; storeCode: string; storeName: string; country: string; items: Item[]; onToast: (s: string) => void }) {
  const { t } = useT();
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
        <button type="button" className="btn btn-primary" onClick={copy} disabled={items.length === 0}>
          <Copy size={16} aria-hidden /> {t("cart.copy")}
        </button>
        <span className="cart__hint">
          <ShoppingCart size={14} aria-hidden /> {t("cart.hint")}
        </span>
      </div>
    </Sheet>
  );
}
