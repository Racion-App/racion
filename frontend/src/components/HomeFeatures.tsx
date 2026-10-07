import { ChevronRight } from "lucide-react";
import { money } from "../lib/format";
import type { Country } from "../lib/types";
import { useT } from "../i18n";

// HomeFeatures — «Что умеет Рацион» под первым вопросом квиза: несколько строк чека с ценой «0»
// и ссылка на страницу возможностей. Квиз остаётся главным, блок — для тех, кто сначала читает.
// Тот же список лежит в HTML главной для роботов (scripts/home-pages.mjs).
const HOME_FEATURES = [2, 3, 4, 5, 7, 11];

export function HomeFeatures({ country }: { country?: Country }) {
  const { t, lang } = useT();
  const zero = money(0, country, lang);
  const prefix = lang === "ru" ? "" : `/${lang}`;
  return (
    <section className="feat feat--home" aria-labelledby="feat-home">
      <h2 className="feat__rtitle" id="feat-home">{t("features.title")}</h2>
      <p className="feat__sub">{t("features.home")}</p>
      <ol className="feat__rows">
        {HOME_FEATURES.map((n) => (
          <li key={n} className="feat__row">
            <div className="feat__item"><h3>{t(`features.${n}.t`)}</h3></div>
            <span className="feat__price num">{zero}</span>
          </li>
        ))}
      </ol>
      <a className="feat__more" href={`${prefix}/features`}>
        {t("features.all")} <ChevronRight size={16} aria-hidden />
      </a>
    </section>
  );
}
