import { useT } from "../i18n";

// Где ребёнок ест по будням: дома, в саду (завтрак, обед и полдник там) или в школе (обед там).
// Планировщик уменьшает порции этих приёмов с понедельника по пятницу, в детском меню пишет «в саду».
export function KidAway({ month, value, onChange }: { month: number; value: string; onChange: (v: string) => void }) {
  const { t } = useT();
  const opts = ["", ...(month < 96 ? ["kindergarten"] : []), ...(month >= 72 ? ["school"] : [])];
  return (
    <div className="kid__row kid__row--away">
      <span>
        {t("kid.away.title")} <small>{t(value ? `kid.away.${value}.sub` : "kid.away.home.sub")}</small>
      </span>
      <div className={`segmented segmented--${opts.length}`} role="radiogroup" aria-label={t("kid.away.title")}>
        {opts.map((o) => (
          <button key={o || "home"} type="button" role="radio" aria-checked={(value || "") === o} onClick={() => onChange(o)}>
            {t(`kid.away.${o || "home"}`)}
          </button>
        ))}
      </div>
    </div>
  );
}
