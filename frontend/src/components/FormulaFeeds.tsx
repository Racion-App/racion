import { useT } from "../i18n";
import { FORMULA_SLOTS } from "../lib/kids";
import type { Child } from "../lib/types";

// Смесь и грудь у ребёнка на смеси — три случая, как в backend/internal/planner/kids.go:
//   - до года только смесь: она во все молочные кормления, спрашивать «когда» незачем;
//   - смешанное вскармливание («Ещё кормлю грудью», до 2 лет): отмечают кормления смесью, в остальные — грудь
//     (Программа вскармливания 2019: грудное молоко в любом сочетании с адаптированной смесью);
//   - с года смесь как напиток: отмечают, когда пьёт; ничего не отмечено — по возрастной норме.
export function FormulaFeeds({ kid, onChange }: { kid: Child; onChange: (patch: Partial<Child>) => void }) {
  const { t } = useT();
  const value = kid.formulaFeeds ?? [];
  const mixed = !!kid.breast && kid.ageMonths < 24;
  const toggle = (s: string) => onChange({ formulaFeeds: value.includes(s) ? value.filter((x) => x !== s) : FORMULA_SLOTS.filter((x) => x === s || value.includes(x)) });
  const showFeeds = mixed || kid.ageMonths >= 12;
  const hint = mixed ? (value.length ? "quiz.formula.when.sub" : "quiz.formula.when.mixed") : value.length ? "quiz.formula.when.toddler" : "quiz.formula.when.toddler.none";
  return (
    <>
      {kid.ageMonths < 24 && (
        <button type="button" className="switch" role="switch" aria-checked={mixed} onClick={() => onChange(mixed ? { breast: false, formulaFeeds: kid.ageMonths < 12 ? [] : value } : { breast: true })}>
          <span className="switch__text">
            {t("quiz.formula.breast")}
            <span className="switch__sub">{t("quiz.formula.breast.sub")}</span>
          </span>
          <span className="switch__track" aria-hidden>
            <span className="switch__knob" />
          </span>
        </button>
      )}
      {showFeeds && (
        <div className="kid__feeds">
          <span className="kid__feedslabel">
            {t("quiz.formula.when")} <small>{t(hint)}</small>
          </span>
          <div className="chips" role="group" aria-label={t("quiz.formula.when")}>
            {FORMULA_SLOTS.map((s) => (
              <button key={s} type="button" className="chip chip--sm" aria-pressed={value.includes(s)} onClick={() => toggle(s)}>
                {t(`formula.feed.${s}`)}
              </button>
            ))}
          </div>
        </div>
      )}
    </>
  );
}
