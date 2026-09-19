import { useEffect, useState } from "react";
import { ExternalLink, X } from "lucide-react";
import { buyLinks, hidePartners, partnersFor, partnersHidden, type BuyLink } from "../lib/partners";
import { track } from "../lib/analytics";
import { useT } from "../i18n";

// Строка «Пригодится: Аэрогриль ↗ · Гриль ↗» под рецептом — только для техники, которую покупают,
// и только если у страны есть магазин. Пока ссылка не партнёрская — подпись «Где купить»;
// с флагом affiliate — «Реклама» и erid. Крестик прячет строку на 30 дней везде.

export function PartnerRow({ equipment, country, onToast }: { equipment: string[]; country?: string; onToast?: (s: string) => void }) {
  const { t, lang } = useT();
  const [links, setLinks] = useState<BuyLink[]>([]);
  const [hidden, setHidden] = useState(partnersHidden);
  useEffect(() => {
    if (!country || hidden) return;
    let alive = true;
    partnersFor(country).then((v) => alive && setLinks(buyLinks(v, equipment)));
    return () => {
      alive = false;
    };
  }, [country, equipment.join(","), hidden]); // eslint-disable-line react-hooks/exhaustive-deps
  if (hidden || links.length === 0) return null;
  const shop = links[0].shop;
  return (
    <div className="partner" lang={lang}>
      <span className="partner__label">{shop.affiliate ? t("partner.ad") : t("partner.where")}</span>
      {links.map((l) => (
        <a key={l.key} className="partner__link" href={l.url} target="_blank" rel="sponsored nofollow noopener" onClick={() => track("partner_click", { code: shop.code, key: l.key })}>
          {t("equipment." + l.key)} <span className="partner__shop">{shop.name}</span>
          <ExternalLink size={12} aria-hidden />
        </a>
      ))}
      {shop.affiliate && shop.erid && <span className="partner__erid">erid: {shop.erid}</span>}
      <button
        type="button"
        className="partner__hide"
        aria-label={t("partner.hide")}
        title={t("partner.hide")}
        onClick={() => {
          hidePartners();
          setHidden(true);
          onToast?.(t("partner.hidden"));
          track("partner_hide");
        }}
      >
        <X size={13} aria-hidden />
      </button>
    </div>
  );
}
