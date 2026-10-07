import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { ExternalLink, Link2, Search, Sparkles, Undo2, X } from "lucide-react";
import { api, ApiError } from "../lib/api";
import { track } from "../lib/analytics";
import { approx } from "../lib/format";
import type { Country, ImportDraft, IngredientRef, Labeled, OwnRecipe, OwnRecipeInput } from "../lib/types";
import { OWN_TAGS, slotLabel } from "../lib/types";
import { locales, useT } from "../i18n";
import { PhotoField } from "./PhotoField";
import { Select } from "./Select";

// Форма своего рецепта: название, приём пищи, время, техника, теги, продукты из базы с количеством, шаги.
// Продукты только из базы — иначе не посчитать калории и цену. Валидация повторяет серверную, сервер решает.
// Новый рецепт можно взять по ссылке: сервер читает страницу и заполняет форму, человек проверяет и сохраняет.

type Props = {
  initial: OwnRecipe | null; // null — новый
  equipment: Labeled[];
  country: Country | undefined;
  ai?: boolean; // помощник включён на сервере
  photos?: boolean; // загрузка фото включена
  importFrom?: string; // "" — открыть с полем ссылки в фокусе, адрес — сразу взять рецепт с этой страницы
  onSaved: (r: OwnRecipe) => void;
  onCancel: () => void;
};

type Row = { ingredientId: string; amount: string; line?: string }; // line — строка с сайта, из которой взят продукт

// searchHint — что подставить в поиск продукта по строке с сайта: первое слово без количества и единиц,
// укороченное до основы («Пармезан — 50 г» → «парме»), чтобы поиск по вхождению нашёл и другие формы слова
function searchHint(line: string): string {
  const word = line
    .replace(/\([^)]*\)/g, " ")
    .split(/[\s,;:—–-]+/)
    .find((w) => /\p{L}{3,}/u.test(w) && !/^(г|гр|кг|мл|л|шт|ст|ч|g|kg|ml|cups?|tbsp|tsp)\.?$/iu.test(w));
  if (!word) return "";
  const w = word.toLowerCase().replace(/[^\p{L}]/gu, "");
  return w.length >= 6 ? w.slice(0, 5) : w.length >= 4 ? w.slice(0, -1) : w;
}

export function OwnRecipeForm({ initial, equipment, country, ai, photos, importFrom, onSaved, onCancel }: Props) {
  const { t, tn, lang } = useT();
  const [title, setTitle] = useState(initial?.title ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [image, setImage] = useState(initial?.image ?? "");
  const [slot, setSlot] = useState(initial?.slot ?? "dinner");
  const [timeMin, setTimeMin] = useState(String(initial?.timeMin ?? 30));
  const [eq, setEq] = useState<string[]>(initial?.equipment ?? ["stove"]);
  const [tags, setTags] = useState<string[]>(initial?.tags ?? []);
  const [rows, setRows] = useState<Row[]>(initial?.ingredients.map((i) => ({ ingredientId: i.ingredientId, amount: String(i.amount) })) ?? []);
  const [source, setSource] = useState(initial?.source ?? "");
  // рецепт по ссылке: адрес, ход запроса и сводка того, что взяли со страницы
  const [url, setUrl] = useState(importFrom ?? "");
  const [importing, setImporting] = useState(false);
  const [draft, setDraft] = useState<ImportDraft | null>(null);
  const urlRef = useRef<HTMLInputElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const [stepsText, setStepsText] = useState(initial?.steps.join("\n") ?? "");
  const [base, setBase] = useState<IngredientRef[] | null>(null);
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const titleRef = useRef<HTMLInputElement>(null);
  // помощник: что было до правки, чтобы вернуть одним нажатием
  const [aiBusy, setAiBusy] = useState(false);
  const [aiNote, setAiNote] = useState<string | null>(null);
  const [before, setBefore] = useState<{ title: string; description: string; stepsText: string } | null>(null);
  // на какой язык переводить: по умолчанию язык интерфейса, можно выбрать любой из поддерживаемых
  const [toLang, setToLang] = useState(lang);

  const assist = async (action: "improve" | "translate", to = toLang) => {
    setError(null);
    setAiNote(null);
    setAiBusy(true);
    try {
      const out = await api.assistRecipe({ action, lang: action === "translate" ? to : lang, title: title.trim(), description: description.trim(), steps: stepsText.split(/\r?\n/).map((s) => s.trim()).filter(Boolean) });
      setBefore({ title, description, stepsText });
      setTitle(out.title);
      setDescription(out.description);
      setStepsText(out.steps.join("\n"));
      setAiNote(t("own.ai.done"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("api.ai.busy"));
    } finally {
      setAiBusy(false);
    }
  };
  const runImport = async (raw: string) => {
    const link = raw.trim();
    if (!link || importing) return;
    setError(null);
    setImporting(true);
    try {
      const d = await api.importRecipe(link);
      setTitle(d.title);
      setDescription(d.description);
      setSlot(d.slot);
      setTimeMin(String(d.timeMin));
      setEq(d.equipment);
      setStepsText(d.steps.join("\n"));
      setRows(d.ingredients.map((i) => ({ ingredientId: i.ingredientId, amount: String(i.amount), line: i.line })));
      setSource(d.source);
      setDraft(d);
      setBefore(null);
      setAiNote(null);
      track("own_recipe_import", { host: d.host, found: d.found, matched: d.matched, items: d.ingredients.length, unmatched: d.unmatched.length });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
      track("own_recipe_import_fail", { host: link.replace(/^https?:\/\//, "").split("/")[0] });
    } finally {
      setImporting(false);
    }
  };

  const undo = () => {
    if (!before) return;
    setTitle(before.title);
    setDescription(before.description);
    setStepsText(before.stepsText);
    setBefore(null);
    setAiNote(null);
  };

  useEffect(() => {
    api.ingredients().then(setBase).catch(() => setBase([]));
    if (importFrom !== undefined) urlRef.current?.focus();
    else titleRef.current?.focus();
    if (importFrom) void runImport(importFrom);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const byId = useMemo(() => new Map((base ?? []).map((i) => [i.id, i])), [base]);
  const suggestions = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (q.length < 2 || !base) return [];
    const taken = new Set(rows.map((r) => r.ingredientId));
    return base.filter((i) => i.label.toLowerCase().includes(q) && !taken.has(i.id)).slice(0, 6);
  }, [query, base, rows]);

  const toggle = (list: string[], id: string, set: (v: string[]) => void) => set(list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);

  const defaultAmount = (ing: IngredientRef) => (ing.unit === "pcs" ? "1" : ing.pantry ? "5" : "100");

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    const body: OwnRecipeInput = {
      title: title.trim(),
      description: description.trim(),
      image,
      slot,
      timeMin: Math.round(Number(timeMin) || 0),
      equipment: eq,
      tags,
      steps: stepsText.split(/\r?\n/).map((s) => s.trim()).filter(Boolean),
      ingredients: rows.map((r) => ({ ingredientId: r.ingredientId, amount: Number(String(r.amount).replace(",", ".")) || 0 })),
      source,
    };
    setSaving(true);
    try {
      const saved = initial ? await api.updateOwnRecipe(initial.id, body, country?.code) : await api.createOwnRecipe(body, country?.code);
      onSaved(saved);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <form className="ownform" onSubmit={submit} aria-label={initial ? t("own.edit") : t("own.new")}>
      <h2 className="ownform__title">{initial ? t("own.edit") : t("own.new")}</h2>

      {!initial && (
        <div className="ownform__import" aria-busy={importing}>
          <label htmlFor="own-url">{t("import.label")}</label>
          <div className="ownform__import-row">
            <input
              ref={urlRef}
              id="own-url"
              className="form-control"
              type="url"
              inputMode="url"
              autoComplete="off"
              placeholder="https://"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => {
                // Enter в поле ссылки берёт рецепт, а не сохраняет форму
                if (e.key === "Enter") {
                  e.preventDefault();
                  void runImport(url);
                }
              }}
            />
            <button type="button" className="btn btn-soft" disabled={importing || !url.trim()} onClick={() => void runImport(url)}>
              <Link2 size={16} aria-hidden /> {importing ? t("import.busy") : t("import.go")}
            </button>
          </div>
          <small role="status">{importing ? t("import.working") : t("import.hint")}</small>
        </div>
      )}

      {draft && (
        <div className="ownform__draft" role="status">
          <p>
            {t("import.done", { host: draft.host })}{" "}
            {t(draft.portionsGuessed ? "import.portions.guess" : "import.portions", { n: draft.portions, portions: tn("portions", draft.portions) })}
          </p>
          {draft.found === "ai" && <p>{t("import.found.ai")}</p>}
          {draft.unmatched.length > 0 && (
            <>
              <p>{t("import.unmatched")}</p>
              <div className="chips">
                {draft.unmatched.map((line) => (
                  <button
                    key={line}
                    type="button"
                    className="chip chip--sm"
                    onClick={() => {
                      setQuery(searchHint(line));
                      searchRef.current?.focus();
                    }}
                  >
                    <Search size={14} aria-hidden /> {line}
                  </button>
                ))}
              </div>
            </>
          )}
          {ai && draft.lang && draft.lang !== lang && locales().some((l) => l.code === draft.lang) && (
            <p>
              {t("import.lang", { lang: locales().find((l) => l.code === draft.lang)?.name ?? draft.lang })}{" "}
              <button type="button" className="btn btn-link btn-sm" disabled={aiBusy} onClick={() => { setToLang(lang); void assist("translate", lang); }}>
                {t("own.ai.translate", { lang: locales().find((l) => l.code === lang)?.name ?? lang })}
              </button>
            </p>
          )}
        </div>
      )}

      <label className="auth__field">
        <span>{t("own.title")}</span>
        <input ref={titleRef} className="form-control" value={title} onChange={(e) => setTitle(e.target.value)} maxLength={80} placeholder={t("own.title.ph")} required />
      </label>

      {photos && (
        <div className="auth__field">
          <span>{t("own.photo")}</span>
          <PhotoField value={image} onChange={(url) => setImage(url)} kind="recipe" />
        </div>
      )}

      <label className="auth__field">
        <span>{t("own.description")}</span>
        <textarea className="form-control" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} maxLength={300} placeholder={t("own.description.ph")} />
      </label>

      <div className="ownform__row">
        <div className="auth__field">
          <span id="own-slot">{t("own.slot")}</span>
          <div className="segmented segmented--4" role="radiogroup" aria-labelledby="own-slot">
            {["breakfast", "lunch", "dinner", "snack"].map((s) => (
              <button key={s} type="button" role="radio" aria-checked={slot === s} onClick={() => setSlot(s)}>
                {slotLabel(lang, s)}
              </button>
            ))}
          </div>
        </div>
        <label className="auth__field ownform__time">
          <span>{t("own.time")}</span>
          <input className="form-control num" type="number" inputMode="numeric" min={1} max={600} value={timeMin} onChange={(e) => setTimeMin(e.target.value)} required />
        </label>
      </div>

      <div className="auth__field">
        <span>{t("own.equipment")}</span>
        <div className="chips">
          {equipment.map((e) => (
            <button key={e.id} type="button" className="chip chip--sm" aria-pressed={eq.includes(e.id)} onClick={() => toggle(eq, e.id, setEq)}>
              {e.label}
            </button>
          ))}
        </div>
        <small>{t("own.equipment.hint")}</small>
      </div>

      <div className="auth__field">
        <span>{t("own.tags")}</span>
        <div className="chips">
          {OWN_TAGS.map((tag) => (
            <button key={tag} type="button" className="chip chip--sm" aria-pressed={tags.includes(tag)} onClick={() => toggle(tags, tag, setTags)}>
              {t(`tag.${tag}`)}
            </button>
          ))}
        </div>
      </div>

      <div className="auth__field">
        <label htmlFor="own-ing">{t("own.ingredients")}</label>
        <small>{t("own.ingredients.hint")}</small>
        {rows.length === 0 && <p className="ownform__empty">{t("own.ingredients.empty")}</p>}
        {rows.length > 0 && (
          <ul className="ownform__ings">
            {rows.map((r) => {
              const ing = byId.get(r.ingredientId);
              const name = ing?.label ?? r.ingredientId;
              const unit = ing?.unit ?? "g";
              return (
                <li key={r.ingredientId} className="ownform__ing">
                  <span className="ownform__ing-name">
                    {name}
                    {r.line && <small className="ownform__ing-src">{t("import.from", { line: r.line.replace(/(\d) (?=\p{L})/gu, "$1 ") })}</small>}
                  </span>
                  <label className="ownform__amount">
                    <span className="visually-hidden">{t("own.amount", { unit: t(`unit.${unit}`) })}</span>
                    <input
                      className="form-control num"
                      type="number"
                      inputMode="decimal"
                      min={0}
                      step="any"
                      value={r.amount}
                      onChange={(e) => setRows(rows.map((x) => (x.ingredientId === r.ingredientId ? { ...x, amount: e.target.value } : x)))}
                    />
                    <span className="ownform__unit" aria-hidden>
                      {t(`unit.${unit}`)}
                    </span>
                  </label>
                  <button type="button" className="planrow__del" aria-label={t("own.remove", { name })} onClick={() => setRows(rows.filter((x) => x.ingredientId !== r.ingredientId))}>
                    <X size={16} aria-hidden />
                  </button>
                </li>
              );
            })}
          </ul>
        )}
        <div className="search">
          <Search size={18} aria-hidden />
          <input ref={searchRef} id="own-ing" className="form-control" placeholder={t("own.ingredients.search")} value={query} onChange={(e) => setQuery(e.target.value)} autoComplete="off" />
        </div>
        {suggestions.length > 0 && (
          <div className="suggest" role="listbox" aria-label={t("own.ingredients.search")}>
            {suggestions.map((s) => (
              <button
                key={s.id}
                type="button"
                role="option"
                aria-selected={false}
                onClick={() => {
                  setRows([...rows, { ingredientId: s.id, amount: defaultAmount(s) }]);
                  setQuery("");
                }}
              >
                {s.label} <span className="suggest__sub">{t(`unit.${s.unit}`)}</span>
              </button>
            ))}
          </div>
        )}
        {query.trim().length >= 2 && suggestions.length === 0 && base && <p className="ownform__empty">{t("own.ingredients.nomatch")}</p>}
      </div>

      <label className="auth__field">
        <span>{t("own.steps")}</span>
        <textarea className="form-control" rows={6} value={stepsText} onChange={(e) => setStepsText(e.target.value)} placeholder={t("own.steps.ph")} required />
        <small>{t("own.steps.hint")}</small>
      </label>

      {ai && (
        <div className="ownform__ai" aria-busy={aiBusy}>
          <div className="ownform__ai-row">
            <button type="button" className="btn btn-soft btn-sm" disabled={aiBusy} onClick={() => assist("improve")}>
              <Sparkles size={16} aria-hidden /> {t("own.ai.improve")}
            </button>
            <span className="ownform__ai-lang">
              <Select value={toLang} onChange={setToLang} searchable className="select--sm" options={locales().map((l) => ({ value: l.code, label: l.name }))} aria-label={t("own.ai.translate.to")} />
              <button type="button" className="btn btn-soft btn-sm" disabled={aiBusy} onClick={() => assist("translate")}>
                <Sparkles size={16} aria-hidden /> {t("own.ai.translate", { lang: locales().find((l) => l.code === toLang)?.name ?? toLang })}
              </button>
            </span>
            {before && (
              <button type="button" className="btn btn-link btn-sm" onClick={undo}>
                <Undo2 size={16} aria-hidden /> {t("own.ai.undo")}
              </button>
            )}
          </div>
          <small role="status">{aiBusy ? t("own.ai.working") : aiNote ?? t("own.ai.hint")}</small>
        </div>
      )}

      {error && (
        <p className="ownform__error" role="alert">
          {error}
        </p>
      )}
      {source && (
        <p className="ownform__source">
          <a href={source} target="_blank" rel="noopener noreferrer nofollow">
            {t("recipe.source", { host: source.replace(/^https?:\/\/(www\.)?/, "").split("/")[0] })} <ExternalLink size={13} aria-hidden />
          </a>
        </p>
      )}
      {initial?.kcal !== undefined && initial.cost !== undefined && (
        <p className="quiz__hint">{t("own.per.portion", { kcal: Math.round(initial.kcal), cost: approx(initial.cost, country, lang) })}</p>
      )}
      <div className="ownform__actions">
        <button type="submit" className="btn btn-primary" disabled={saving}>
          {initial ? t("own.save") : t("own.create")}
        </button>
        <button type="button" className="btn btn-soft" onClick={onCancel}>
          {t("cancel")}
        </button>
      </div>
    </form>
  );
}
