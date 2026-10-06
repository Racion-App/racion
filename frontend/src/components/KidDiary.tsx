import { useT } from "../i18n";
import type { Child, WeaningFood } from "../lib/types";

// Дневник прикорма в карточке ребёнка (кабинет → Семья): что и когда ввели, на что была реакция.
// Отметки ставятся в плане кнопками «Ввели, всё хорошо» / «Была реакция»; здесь их видно и можно снять.
export function KidDiary({ kid, foods, onUndo }: { kid: Child; foods: WeaningFood[]; onUndo?: (food: string) => void }) {
  const { t, lang } = useT();
  if (!kid.diary?.length) return null;
  const name = (id: string) => foods.find((f) => f.id === id)?.name ?? id;
  const day = (d: string) => {
    try {
      return new Intl.DateTimeFormat(lang, { day: "numeric", month: "short" }).format(new Date(d + "T12:00:00"));
    } catch {
      return d;
    }
  };
  return (
    <div className="kiddiary">
      <span className="kiddiary__title">{t("account.diary.title")}</span>
      <ul className="kiddiary__list">
        {kid.diary.map((e) => (
          <li key={e.food} className={"kiddiary__row is-" + e.how}>
            <span className="kiddiary__date num">{day(e.date)}</span>
            <span className="kiddiary__food">{name(e.food)}</span>
            <span className="kiddiary__how">{t("account.diary." + e.how)}</span>
            {onUndo && (
              <button type="button" className="kiddiary__undo" onClick={() => onUndo(e.food)} aria-label={`${t("account.diary.undo")}: ${name(e.food)}`}>
                {t("account.diary.undo")}
              </button>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
