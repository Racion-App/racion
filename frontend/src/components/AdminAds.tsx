import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { AdminOffers } from "./AdminOffers";
import { AdminPartners } from "./AdminPartners";
import { useT } from "../i18n";

// Вкладка «Реклама»: общий выключатель (пока договорённостей нет — всё выключено), ниже точечные
// предложения и партнёрские магазины. Выключатель прячет и ссылки «где купить», и доставки, и предложения.

export function AdminAds({ onToast }: { onToast: (s: string) => void }) {
  const { t } = useT();
  const [on, setOn] = useState<boolean | null>(null);
  useEffect(() => {
    api.adminAds().then((v) => setOn(v.enabled)).catch((e: Error) => onToast(e.message));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const toggle = async () => {
    if (on === null) return;
    try {
      const v = await api.adminSetAds(!on);
      setOn(v.enabled);
      onToast(v.enabled ? t("admin.ads.on.toast") : t("admin.ads.off.toast"));
    } catch (e) {
      onToast((e as Error).message);
    }
  };
  return (
    <>
      <section className="admin__section">
        <button type="button" className="switch switch--big" role="switch" aria-checked={!!on} disabled={on === null} onClick={toggle}>
          <span className="switch__text">
            {t("admin.ads.enabled")}
            <small>{on ? t("admin.ads.on.hint") : t("admin.ads.off.hint")}</small>
          </span>
          <span className="switch__track" aria-hidden><span className="switch__knob" /></span>
        </button>
      </section>
      <h2 className="admin__h2">{t("admin.offers")}</h2>
      <AdminOffers onToast={onToast} />
      <h2 className="admin__h2">{t("admin.partners.stores")}</h2>
      <AdminPartners onToast={onToast} />
    </>
  );
}
