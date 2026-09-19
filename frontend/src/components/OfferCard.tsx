import { useEffect, useState } from "react";
import { Copy, Check, ExternalLink } from "lucide-react";
import { api } from "../lib/api";
import type { Offer } from "../lib/types";
import { track } from "../lib/analytics";
import { useT } from "../i18n";

// Точечное предложение: картинка товара, заголовок, промокод с копированием и кнопка перехода.
// Подпись «Реклама» и erid обязательны для партнёрских (закон о рекламе РФ). Одно предложение на место.

export function useOffer(place: Offer["place"], country: string | undefined, region?: string, match?: string[]) {
  const [offer, setOffer] = useState<Offer | null>(null);
  const key = (match ?? []).join(",");
  useEffect(() => {
    if (!country) return;
    let alive = true;
    api.offers(place, country, region, match).then((list) => alive && setOffer(list[0] ?? null)).catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [place, country, region, key]); // eslint-disable-line react-hooks/exhaustive-deps
  return offer;
}

export function OfferCard({ offer, compact }: { offer: Offer; compact?: boolean }) {
  const { t } = useT();
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(offer.promo);
      setCopied(true);
      track("offer_promo_copy", { id: offer.id });
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // без буфера обмена промокод всё равно виден
    }
  };
  return (
    <div className={"offer offer--card" + (compact ? " offer--compact" : "")}>
      {offer.image && <img className="offer__img" src={offer.image} alt="" width={72} height={72} loading="lazy" />}
      <div className="offer__body">
        <span className="offer__label">{offer.affiliate ? t("partner.ad") : t("partner.need")}</span>
        <b className="offer__title">{offer.title}</b>
        {offer.body && <span className="offer__text">{offer.body}</span>}
        {offer.promo && (
          <button type="button" className="offer__promo offer__promo--btn" onClick={copy} aria-label={t("offer.copy")}>
            {t("offer.promo")} <code>{offer.promo}</code> {copied ? <Check size={14} aria-hidden /> : <Copy size={14} aria-hidden />}
          </button>
        )}
        <a className="offer__cta" href={offer.url} target="_blank" rel="sponsored nofollow noopener" onClick={() => track("offer_click", { id: offer.id })}>
          {offer.cta || t("offer.open")} <ExternalLink size={13} aria-hidden />
        </a>
        {offer.erid && <span className="partner__erid">erid: {offer.erid}</span>}
      </div>
    </div>
  );
}
