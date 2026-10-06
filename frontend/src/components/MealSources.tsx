import { useT } from "../i18n";
import { mealSlots, mealSourceOptions } from "../lib/kids";

// Режим «комбинирую»: для каждого приёма своё — готовим сами, баночки или с общего стола.
// Не выбрано — «сами» (так же решает бэкенд, planner.Child.normalized).
export function MealSources({ month, value, onChange }: { month: number; value: Record<string, string>; onChange: (v: Record<string, string>) => void }) {
  const { t } = useT();
  const opts = mealSourceOptions(month);
  return (
    <div className="mealsrc">
      <span className="mealsrc__hint">{t("quiz.mix.hint")}</span>
      {mealSlots(month).map((s) => {
        const cur = value[s] || "home";
        return (
          <div className="mealsrc__row" key={s}>
            <span className="mealsrc__slot">{t(`slot.${s}`)}</span>
            <div className={`segmented segmented--${opts.length}`} role="radiogroup" aria-label={t(`slot.${s}`)}>
              {opts.map((o) => (
                <button key={o} type="button" role="radio" aria-checked={cur === o} onClick={() => onChange({ ...value, [s]: o })}>
                  {t(`meal.src.${o}`)}
                </button>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
