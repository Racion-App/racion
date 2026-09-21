import { useEffect, useState } from "react";
import { ReceiptText } from "lucide-react";
import { Link } from "react-router-dom";
import { LangButton } from "./LangDialog";
import { useT } from "../i18n";
import { api } from "../lib/api";

// Подборки-вопросы в подвале каждой страницы: самые частые запросы, те же, что в серверном подвале
const FEATURED = ["dinner-ideas", "birthday-table", "new-year-table", "kids-party"];

// Подвал сайта: тот же, что у серверных страниц (layout.html «foot»), чтобы ссылки на каталог,
// кабинет и юридические страницы были на каждом экране приложения.
export function SiteFooter() {
  const { t, lang } = useT();
  const p = lang === "ru" ? "" : `/${lang}`;
  const [featured, setFeatured] = useState<{ slug: string; name: string }[]>([]);
  useEffect(() => {
    let alive = true;
    api.publicCollections(lang)
      .then((cs) => { if (alive) setFeatured(FEATURED.map((slug) => cs.find((c) => c.slug === slug)).filter((c): c is NonNullable<typeof c> => !!c).map((c) => ({ slug: c.slug!, name: c.name }))); })
      .catch(() => {});
    return () => { alive = false; };
  }, [lang]);
  return (
    <footer className="sitefoot">
      <div className="sitefoot__top">
        <Link className="sitefoot__brand" to="/">
          <ReceiptText size={20} strokeWidth={2.2} aria-hidden />
          {t("page.brand")}
        </Link>
        <p className="sitefoot__tag">{t("foot.tag")}</p>
        <Link className="btn btn-primary sitefoot__cta" to="/">{t("page.foot.plan")}</Link>
      </div>
      <nav className="sitefoot__cols" aria-label={t("foot.nav")}>
        <div className="sitefoot__col">
          <h3>{t("foot.recipes")}</h3>
          <a href={`${p}/recipes`}>{t("page.foot.recipes")}</a>
          <a href={`${p}/recipes?slot=breakfast`}>{t("slot.breakfast")}</a>
          <a href={`${p}/recipes?slot=lunch`}>{t("slot.lunch")}</a>
          <a href={`${p}/recipes?slot=dinner`}>{t("slot.dinner")}</a>
          <a href={`${p}/recipes?tag=kidmenu`}>{t("foot.kids")}</a>
        </div>
        {featured.length > 0 && (
          <div className="sitefoot__col">
            <h3>{t("foot.cook")}</h3>
            {featured.map((c) => <a key={c.slug} href={`${p}/collection/${c.slug}`}>{c.name}</a>)}
            <a href={`${p}/collections`}>{t("foot.cook.all")}</a>
          </div>
        )}
        <div className="sitefoot__col">
          <h3>{t("foot.app")}</h3>
          <Link to="/">{t("page.foot.plan")}</Link>
          <Link to="/me">{t("page.account")}</Link>
          <Link to="/me?tab=recipes">{t("account.recipes")}</Link>
          <Link to="/me?tab=family">{t("account.family")}</Link>
        </div>
        <div className="sitefoot__col">
          <h3>{t("foot.about")}</h3>
          <p>{t("foot.free")}</p>
          <p>{t("page.foot")}</p>
          <a href={`${p}/terms`}>{t("legal.terms")}</a>
          <a href={`${p}/privacy`}>{t("legal.privacy")}</a>
          <a href={`${p}/status`}>{t("status.title")}</a>
          <LangButton footer />
        </div>
      </nav>
      <p className="sitefoot__copy">© {t("page.brand")}</p>
    </footer>
  );
}
