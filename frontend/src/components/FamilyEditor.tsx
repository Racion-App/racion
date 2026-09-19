import { useMemo } from "react";
import { Baby, Plus, Trash2, User } from "lucide-react";
import { Select } from "./Select";
import type { Child, Member, Meta } from "../lib/types";
import { APPETITES } from "../lib/types";
import { ageOptions, feedingOptions, formulaMlByAge } from "../lib/kids";
import { useT } from "../i18n";

// Состав семьи: взрослые (имя, цель, аппетит, приёмы дома) и дети (имя, возраст, кормление, смесь).
// Одна разметка для раздела «Семья» в кабинете; квиз подставляет этот состав как ответы.

type Props = {
  adults: Member[];
  kids: Child[];
  meta: Meta | null;
  onChange: (next: { adults: Member[]; kids: Child[] }) => void;
};

export const EMPTY_ADULT: Member = { name: "", goal: "none", appetite: "normal", slots: [] };
export const EMPTY_KID: Child = { name: "", ageMonths: 60, feeding: "shared", sharesMeals: false, formula: false, formulaBrand: "", formulaMl: 0 };

export function FamilyEditor({ adults, kids, meta, onChange }: Props) {
  const { t, lang } = useT();
  const AGES = useMemo(() => ageOptions(t, lang), [t, lang]);
  const slots = meta?.slots ?? [];
  const setAdult = (i: number, patch: Partial<Member>) => onChange({ adults: adults.map((m, j) => (j === i ? { ...m, ...patch } : m)), kids });
  const setKid = (i: number, patch: Partial<Child>) => onChange({ adults, kids: kids.map((k, j) => (j === i ? { ...k, ...patch } : k)) });
  const toggleSlot = (i: number, slot: string) => {
    const m = adults[i];
    const all = slots.map((s) => s.id);
    const cur = m.slots.length === 0 ? all : m.slots;
    const next = cur.includes(slot) ? cur.filter((x) => x !== slot) : [...cur, slot];
    if (next.length === 0) return;
    setAdult(i, { slots: next.length === all.length ? [] : next });
  };

  return (
    <div className="family">
      <div className="family__group">
        {adults.map((m, i) => (
          <div className="kid member" key={i}>
            <div className="kid__head">
              <label className="member__name">
                <User size={18} aria-hidden />
                <input className="form-control" value={m.name} maxLength={40} placeholder={t("quiz.member.name", { n: i + 1 })} onChange={(e) => setAdult(i, { name: e.target.value })} aria-label={t("quiz.member.name.aria", { n: i + 1 })} />
              </label>
              <button type="button" className="kid__remove" onClick={() => onChange({ adults: adults.filter((_, j) => j !== i), kids })} aria-label={t("quiz.member.remove")}>
                <Trash2 size={16} aria-hidden />
              </button>
            </div>
            <div className="member__row">
              <span className="member__label">{t("quiz.goal")}</span>
              <div className="chips" role="radiogroup" aria-label={t("quiz.goal")}>
                {(meta?.goals ?? []).map((g) => (
                  <button key={g.id} type="button" role="radio" className="chip chip--sm" aria-checked={m.goal === g.id} aria-pressed={m.goal === g.id} onClick={() => setAdult(i, { goal: g.id })}>
                    {g.label}
                  </button>
                ))}
              </div>
            </div>
            <div className="member__row">
              <span className="member__label">{t("quiz.appetite")}</span>
              <div className="segmented segmented--3 member__seg" role="radiogroup" aria-label={t("quiz.appetite")}>
                {APPETITES.map((a) => (
                  <button key={a} type="button" role="radio" aria-checked={m.appetite === a} onClick={() => setAdult(i, { appetite: a })}>
                    {t(`appetite.${a}`)}
                  </button>
                ))}
              </div>
            </div>
            <div className="member__row">
              <span className="member__label">{t("quiz.member.slots")}</span>
              <div className="chips" role="group" aria-label={t("quiz.member.slots")}>
                {slots.map((s) => {
                  const on = m.slots.length === 0 || m.slots.includes(s.id);
                  return (
                    <button key={s.id} type="button" className="chip chip--sm" aria-pressed={on} onClick={() => toggleSlot(i, s.id)}>
                      {s.label}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        ))}
        {adults.length < 8 && (
          <button type="button" className="btn btn-soft" onClick={() => onChange({ adults: [...adults, { ...EMPTY_ADULT }], kids })}>
            <Plus size={18} aria-hidden /> {t("quiz.member.add")}
          </button>
        )}
      </div>

      <div className="family__group">
        {kids.map((k, i) => (
          <div className="kid" key={i}>
            <div className="kid__head">
              <label className="member__name">
                <Baby size={18} aria-hidden />
                <input className="form-control" value={k.name ?? ""} maxLength={40} placeholder={`${t("quiz.child")} ${kids.length > 1 ? i + 1 : ""}`.trim()} onChange={(e) => setKid(i, { name: e.target.value })} aria-label={t("family.kid.name", { n: i + 1 })} />
              </label>
              <button type="button" className="kid__remove" onClick={() => onChange({ adults, kids: kids.filter((_, j) => j !== i) })} aria-label={t("quiz.child.remove")}>
                <Trash2 size={16} aria-hidden />
              </button>
            </div>
            <label className="kid__row">
              <span>{t("quiz.age")}</span>
              <Select
                aria-label={t("quiz.age")}
                value={String(k.ageMonths)}
                options={AGES.map((a) => ({ value: String(a.months), label: a.label }))}
                onChange={(v) => {
                  const m = Number(v);
                  const opts = feedingOptions(m);
                  setKid(i, { ageMonths: m, feeding: opts.includes(k.feeding) ? k.feeding : opts[0], sharesMeals: false, formula: m < 36 ? k.formula : false });
                }}
              />
            </label>
            {k.ageMonths >= 6 && (
              <div className="kid__feeding" role="radiogroup" aria-label={t("quiz.feeding")}>
                {feedingOptions(k.ageMonths).map((f) => (
                  <button key={f} type="button" role="radio" aria-checked={k.feeding === f} className="option option--compact" onClick={() => setKid(i, { feeding: f })}>
                    <span>
                      <span className="option__title">{t(`feeding.${f}`)}</span>
                      <span className="option__sub">{t(`feeding.${f}.sub`)}</span>
                    </span>
                  </button>
                ))}
              </div>
            )}
            {k.ageMonths < 36 && (
              <button type="button" className="switch" role="switch" aria-checked={k.formula} onClick={() => setKid(i, { formula: !k.formula, formulaBrand: !k.formula && !k.formulaBrand ? "nutrilon" : k.formulaBrand })}>
                <span className="switch__text">
                  {t("quiz.formula")}
                  <span className="switch__sub">{k.ageMonths < 6 ? t("quiz.formula.sub.young") : t("quiz.formula.sub")}</span>
                </span>
                <span className="switch__track" aria-hidden>
                  <span className="switch__knob" />
                </span>
              </button>
            )}
            {k.formula && k.ageMonths < 36 && (
              <>
                <label className="kid__row">
                  <span>{t("quiz.formula.brand")}</span>
                  <Select aria-label={t("quiz.formula.brand")} value={k.formulaBrand} onChange={(v) => setKid(i, { formulaBrand: v })} options={(meta?.formulaBrands ?? []).map((b) => ({ value: b.id, label: b.name, sub: b.note }))} />
                </label>
                <label className="kid__row">
                  <span>
                    {t("quiz.formula.ml")} <small>{t("quiz.formula.ml.sub", { n: formulaMlByAge(k.ageMonths) })}</small>
                  </span>
                  <input className="form-control kid__num" type="number" inputMode="numeric" min={100} max={1500} step={50} placeholder={String(formulaMlByAge(k.ageMonths))} value={k.formulaMl || ""} onChange={(e) => setKid(i, { formulaMl: Number(e.target.value) || 0 })} />
                </label>
              </>
            )}
          </div>
        ))}
        {kids.length < 8 && (
          <button type="button" className="btn btn-soft" onClick={() => onChange({ adults, kids: [...kids, { ...EMPTY_KID }] })}>
            <Plus size={18} aria-hidden /> {t("quiz.child.add")}
          </button>
        )}
      </div>
    </div>
  );
}
