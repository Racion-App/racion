import { ChevronDown } from "lucide-react";
import { useT } from "../i18n";

// Аллергии самого ребёнка: прикорм, детское меню и общие блюда (если ест со стола) их учитывают.
// Свёрнуто в одну строку, чтобы карточка ребёнка не разрасталась: в заголовке — что уже отмечено.
export function KidAllergens({ options, value, onChange }: { options: { id: string; label: string }[]; value: string[]; onChange: (v: string[]) => void }) {
  const { t } = useT();
  const toggle = (id: string) => onChange(value.includes(id) ? value.filter((x) => x !== id) : [...value, id]);
  const picked = options.filter((o) => value.includes(o.id)).map((o) => o.label);
  return (
    <details className="kidall" open={value.length > 0 || undefined}>
      <summary className="kidall__head">
        <span>
          {t("quiz.kid.allergens")} <small>{picked.length > 0 ? picked.join(", ") : t("quiz.kid.allergens.none")}</small>
        </span>
        <ChevronDown size={18} aria-hidden className="kidall__chev" />
      </summary>
      <p className="kidall__hint">{t("quiz.kid.allergens.sub")}</p>
      <div className="chips" role="group" aria-label={t("quiz.kid.allergens")}>
        {options.map((o) => (
          <button key={o.id} type="button" className="chip chip--sm" aria-pressed={value.includes(o.id)} onClick={() => toggle(o.id)}>
            {o.label}
          </button>
        ))}
      </div>
    </details>
  );
}
