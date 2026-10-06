import { useT } from "../i18n";

// Пол ребёнка — только с 11 лет: по нормам МР 2.3.1.0253-21 у мальчиков и девочек 11–17 лет разные калории и
// белок (2500 и 2300 ккал в 11–14 лет, 2900 и 2500 — в 15–17), от этого порция за общим столом и норма в меню.
export function KidSex({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const { t } = useT();
  return (
    <div className="kid__row">
      <span>
        {t("kid.sex")} <small>{t("kid.sex.sub")}</small>
      </span>
      <div className="segmented segmented--3" role="radiogroup" aria-label={t("kid.sex")}>
        {(["m", "f", ""] as const).map((s) => (
          <button key={s || "none"} type="button" role="radio" aria-checked={(value || "") === s} onClick={() => onChange(s)}>
            {t(`kid.sex.${s || "none"}`)}
          </button>
        ))}
      </div>
    </div>
  );
}
