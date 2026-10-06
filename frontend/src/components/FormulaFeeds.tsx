import { useT } from "../i18n";
import { FORMULA_SLOTS } from "../lib/kids";

// Когда малыш получает смесь: только утром, только на ночь, утром и ночью и т. д. Ничего не отмечено —
// смесь во все кормления. От отметок зависят объём в день и подписи кормлений в плане прикорма.
export function FormulaFeeds({ value, onChange }: { value: string[]; onChange: (v: string[]) => void }) {
  const { t } = useT();
  const toggle = (s: string) => onChange(value.includes(s) ? value.filter((x) => x !== s) : FORMULA_SLOTS.filter((x) => x === s || value.includes(x)));
  return (
    <div className="kid__feeds">
      <span className="kid__feedslabel">
        {t("quiz.formula.when")} <small>{value.length === 0 ? t("quiz.formula.when.all") : t("quiz.formula.when.sub")}</small>
      </span>
      <div className="chips" role="group" aria-label={t("quiz.formula.when")}>
        {FORMULA_SLOTS.map((s) => (
          <button key={s} type="button" className="chip chip--sm" aria-pressed={value.includes(s)} onClick={() => toggle(s)}>
            {t(`formula.feed.${s}`)}
          </button>
        ))}
      </div>
    </div>
  );
}
