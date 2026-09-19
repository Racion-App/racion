import { Check, Languages } from "lucide-react";
import { useState } from "react";
import { Sheet } from "./Sheet";
import { locales, localeMeta, useLang, useT } from "../i18n";

// Кнопка-переводчик в шапке и лист выбора языка: флаг, название на самом языке, английское название,
// у неполных переводов — процент. Список приходит из /api/locales, то есть из папки backend/locales.

export function LangButton() {
  const { t, lang } = useT();
  const { setLang, auto } = useLang();
  const [open, setOpen] = useState(false);
  const list = locales();
  const full = localeMeta("ru")?.keys ?? 0;
  return (
    <>
      <button type="button" className="theme-btn theme-btn--lang" onClick={() => setOpen(true)} aria-label={t("lang")} title={`${t("lang")}: ${localeMeta(lang)?.name ?? lang}`}>
        <Languages size={18} aria-hidden />
        <span className={`fi fi-${localeMeta(lang)?.flag ?? "ru"} theme-btn__flag`} aria-hidden />
      </button>
      <Sheet open={open} onClose={() => setOpen(false)} title={t("lang.title")} closeLabel={t("close")}>
        <h2>{t("lang.title")}</h2>
        <p className="bsheet__lead">{auto ? t("lang.auto") : t("lang.hint")}</p>
        <ul className="langlist" role="listbox" aria-label={t("lang")}>
          {list.map((l) => {
            const pct = full ? Math.min(100, Math.round((l.keys / full) * 100)) : 100;
            return (
              <li key={l.code}>
                <button
                  type="button"
                  role="option"
                  aria-selected={l.code === lang}
                  className={"langlist__item" + (l.code === lang ? " is-on" : "")}
                  lang={l.code}
                  onClick={() => {
                    setLang(l.code);
                    setOpen(false);
                  }}
                >
                  <span className={`fi fi-${l.flag} langlist__flag`} aria-hidden />
                  <span className="langlist__name">
                    {l.name}
                    <small>{l.english}{pct < 95 ? ` · ${pct}%` : ""}</small>
                  </span>
                  {l.code === lang && <Check size={18} aria-hidden />}
                </button>
              </li>
            );
          })}
        </ul>
        <p className="langlist__foot">{t("lang.contribute")}</p>
      </Sheet>
    </>
  );
}
