import { Baby, Info, Sparkles } from "lucide-react";
import { useT } from "../i18n";
import type { Weaning, WeaningFeed, WeaningFood, WeaningItem } from "../lib/types";

// Прикорм по месяцам: отметки «уже ест» в квизе и неделя прикорма на странице плана.
// Сроки, граммы и правила — из backend/internal/planner/weaning.go (программа вскармливания 2019).

const GROUPS = ["veg", "cereal", "meat", "fruit", "yolk", "curd", "kefir", "fish", "bread"];

// WeaningPicker — что малыш уже ест: продукты, которые по возрасту уже можно, по группам.
export function WeaningPicker({ foods, month, value, onChange }: { foods: WeaningFood[]; month: number; value: string[]; onChange: (v: string[]) => void }) {
  const { t } = useT();
  const allowed = foods.filter((f) => f.from <= month);
  const toggle = (id: string) => onChange(value.includes(id) ? value.filter((x) => x !== id) : [...value, id]);
  return (
    <div className="weanpick">
      <div className="weanpick__head">
        <b>{t("weaning.pick.title")}</b>
        <span>{t("weaning.pick.hint")}</span>
      </div>
      {GROUPS.map((g) => {
        const list = allowed.filter((f) => f.group === g);
        if (list.length === 0) return null;
        return (
          <div className="weanpick__group" key={g}>
            <span className="weanpick__label">{t(`weaning.group.${g}`)}</span>
            <div className="chips" role="group" aria-label={t(`weaning.group.${g}`)}>
              {list.map((f) => (
                <button key={f.id} type="button" className="chip chip--sm chip--img" aria-pressed={value.includes(f.id)} onClick={() => toggle(f.id)}>
                  {f.img ? <img src={f.img} alt="" loading="lazy" /> : <span className="chip__dot" aria-hidden />}
                  {f.name}
                </button>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function amount(t: (k: string, p?: Record<string, string | number>) => string, it: WeaningItem) {
  if (it.unit === "pcs") return it.grams === 0.25 ? "¼" : it.grams === 0.5 ? "½" : String(it.grams);
  return `${it.grams} ${t(it.unit === "ml" ? "unit.ml" : "unit.g")}`;
}

function Feed({ feed, t }: { feed: WeaningFeed; t: (k: string, p?: Record<string, string | number>) => string }) {
  return (
    <li className="weanfeed">
      <span className="weanfeed__time num">{feed.time}</span>
      <span className="weanfeed__what">
        {feed.items.map((it, i) => (
          <span key={i} className={"weanfeed__item" + (it.new ? " is-new" : "")}>
            {it.name} <span className="num">{amount(t, it)}</span>
            {it.new && <span className="weanfeed__badge">{t("weaning.new.badge")}</span>}
          </span>
        ))}
        {feed.milk && <span className="weanfeed__milk">{feed.items.length > 0 ? t("weaning.milk.after") : t("weaning.milk")}</span>}
      </span>
    </li>
  );
}

// WeaningDay — кормления одного дня в карточке дня плана.
export function WeaningDayRows({ w, day, ageLabel }: { w: Weaning; day: number; ageLabel: string }) {
  const { t } = useT();
  const d = w.days[day];
  if (!d) return null;
  return (
    <div className="kidrows kidrows--wean">
      <div className="kidrows__head">
        <Baby size={13} aria-hidden /> {t("weaning.day.title", { age: ageLabel })}
      </div>
      <ol className="weanfeeds">
        {d.feeds.map((f) => (
          <Feed key={f.time} feed={f} t={t} />
        ))}
      </ol>
    </div>
  );
}

// WeaningCard — сводка недели: продукт недели с наращиванием по дням, что дальше, правила и источник.
export function WeaningCard({ w, ageLabel, labels }: { w: Weaning; ageLabel: string; labels: string[] }) {
  const { t } = useT();
  return (
    <section className="weancard" aria-label={t("weaning.title", { age: ageLabel })}>
      <h2 className="weancard__title">
        <Baby size={18} aria-hidden /> {t("weaning.title", { age: ageLabel })}
      </h2>
      {w.new && w.ramp ? (
        <div className="weancard__new">
          <div className="weancard__newhead">
            <Sparkles size={16} aria-hidden /> {t("weaning.new.title", { name: w.new.name })}
          </div>
          <p className="weancard__p">{t("weaning.new.how")}</p>
          <ol className="weanramp">
            {w.ramp.map((g, i) => (
              <li key={i}>
                <span>{labels[i] ?? i + 1}</span>
                <b className="num">{amount(t, { ...w.new!, grams: g })}</b>
              </li>
            ))}
          </ol>
        </div>
      ) : (
        <p className="weancard__p">{t("weaning.new.none")}</p>
      )}
      {w.next.length > 0 && <p className="weancard__p">{t("weaning.next", { list: w.next.join(", ") })}</p>}
      <ul className="weancard__rules">
        <li>{t(w.texture === "mashed" ? "weaning.rule.mashed" : "weaning.rule.puree")}</li>
        <li>{t("weaning.rule.one")}</li>
        <li>{t("weaning.rule.health")}</li>
        <li>{t("weaning.rule.water")}</li>
        {w.milkMl > 0 && <li>{t("weaning.rule.formula", { n: w.milkMl })}</li>}
      </ul>
      <p className="weancard__src">
        <Info size={13} aria-hidden /> {t("weaning.source")}
      </p>
    </section>
  );
}
