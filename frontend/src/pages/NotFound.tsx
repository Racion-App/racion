import { Link } from "react-router-dom";
import { SearchX } from "lucide-react";
import { SiteFooter } from "../components/SiteFooter";
import { TopBar } from "../components/TopBar";
import { useT } from "../i18n";

// Неизвестный адрес внутри приложения: та же страница ошибки, что и у серверных страниц.
export function NotFound() {
  const { t, lang } = useT();
  const prefix = lang === "ru" ? "" : `/${lang}`;
  return (
    <div className="shell">
      <TopBar />
      <main className="state errpage">
        <div className="state__box">
          <span className="errpage__icon" aria-hidden>
            <SearchX size={28} />
          </span>
          <span className="errpage__code num">404</span>
          <h2>{t("page.404.title")}</h2>
          <p>{t("page.404.text")}</p>
          <div className="errpage__actions">
            <a className="btn btn-primary" href={`${prefix}/recipes`}>
              {t("page.foot.recipes")}
            </a>
            <Link className="btn btn-soft" to="/">
              {t("page.err.home")}
            </Link>
          </div>
        </div>
      </main>
      <SiteFooter />
    </div>
  );
}
