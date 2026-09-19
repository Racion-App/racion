import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Plus, Search, Sparkles, Trash2, Undo2, X } from "lucide-react";
import { EmptyState } from "./EmptyState";
import { api, ApiError } from "../lib/api";
import type { AdminRecipe, CatalogRecipeInput, IngredientRef, Labeled } from "../lib/types";
import { PhotoField } from "./PhotoField";
import { Select } from "./Select";
import { useConfirm } from "./Confirm";
import { AList, ARow } from "./AdminList";
import { slotLabel } from "../lib/types";
import { useT } from "../i18n";

// Рецепты базы в админке: поиск по названию или id, форма правки/создания, удаление.
// После сохранения сервер перечитывает каталог: правка видна на сайте сразу.

type Row = { ingredientId: string; amount: string };

export function AdminRecipes({ equipment, photos, ai, onToast, editId, onOpen, onClose }: { equipment: Labeled[]; photos: boolean; ai?: boolean; onToast: (s: string) => void; editId?: string; onOpen: (id: string) => void; onClose: () => void }) {
  const { t, lang } = useT();
  const confirm = useConfirm();
  const [q, setQ] = useState("");
  const [items, setItems] = useState<AdminRecipe[]>([]);
  const [tags, setTags] = useState<string[]>([]);
  // что редактируем — из адреса: new или id; сам рецепт подгружаем, если открыли по прямой ссылке
  const [editing, setEditing] = useState<AdminRecipe | null | "new">(null);
  const load = (query: string) => api.adminRecipes(query).then((r) => { setItems(r.items); setTags(r.tags); }).catch((e: Error) => onToast(e.message));
  useEffect(() => {
    const id = window.setTimeout(() => load(q), 250);
    return () => window.clearTimeout(id);
  }, [q]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!editId) {
      setEditing(null);
      return;
    }
    if (editId === "new") {
      setEditing("new");
      return;
    }
    const known = items.find((r) => r.id === editId);
    if (known) setEditing(known);
    else api.adminRecipe(editId).then(setEditing).catch((e: Error) => { onToast(e.message); onClose(); });
  }, [editId]); // eslint-disable-line react-hooks/exhaustive-deps

  if (editId && editing !== null) {
    return (
      <RecipeForm
        key={editId}
        initial={editing === "new" ? null : editing}
        equipment={equipment}
        knownTags={tags}
        photos={photos}
        ai={ai}
        onSaved={(rc) => {
          onToast(t("admin.recipe.saved", { title: rc.title }));
          void load(q);
          onClose();
        }}
        onCancel={onClose}
      />
    );
  }

  return (
    <section className="admin__section">
      <div className="admin__row">
        <div className="search admin__search">
          <Search size={18} aria-hidden />
          <input className="form-control" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t("admin.recipe.search")} aria-label={t("admin.recipe.search")} />
        </div>
        <button type="button" className="btn btn-primary" onClick={() => onOpen("new")}>
          <Plus size={16} aria-hidden /> {t("admin.recipe.new")}
        </button>
      </div>
      <AList>
        {items.map((r) => (
          <ARow
            key={r.id}
            media={r.image ? <img className="alist__thumb" src={r.image} alt="" loading="lazy" /> : <span className={"alist__thumb alist__thumb--" + r.slot} aria-hidden />}
            title={r.title}
            meta={`${slotLabel(lang, r.slot)} · ${r.timeMin} ${t("min")} · ${r.ingredients.length} ${t("admin.recipe.ings")} · ${r.steps.length} ${t("admin.recipe.steps")}${r.batch ? ` · ${t("admin.recipe.batch.short")}` : ""}`}
            tags={r.tags.length > 0 && r.tags.map((tg) => <span key={tg} className="tagchip">{tg}</span>)}
            onClick={() => onOpen(r.id)}
            right={
              <a className="alist__id" href={`/recipe/${encodeURIComponent(r.id)}`} target="_blank" rel="noopener" onClick={(e) => e.stopPropagation()}>
                {r.id}
              </a>
            }
            actions={
              <button
                type="button"
                className="planrow__del"
                aria-label={t("delete")}
                onClick={async () => {
                  if (!(await confirm({ title: t("admin.recipe.confirm", { title: r.title }), text: t("admin.recipe.confirm.text"), ok: t("delete"), danger: true }))) return;
                  try {
                    await api.adminDeleteRecipe(r.id);
                    onToast(t("admin.recipe.deleted"));
                    void load(q);
                  } catch (e) {
                    onToast((e as Error).message);
                  }
                }}
              >
                <Trash2 size={16} aria-hidden />
              </button>
            }
          />
        ))}
      </AList>
      {items.length === 0 && <EmptyState icon={<Search size={20} />} text={t("catalog.empty")} hint={t("catalog.empty.hint")} />}
    </section>
  );
}

function RecipeForm({ initial, equipment, knownTags, photos, ai, onSaved, onCancel }: { initial: AdminRecipe | null; equipment: Labeled[]; knownTags: string[]; photos: boolean; ai?: boolean; onSaved: (r: AdminRecipe) => void; onCancel: () => void }) {
  const { t, lang } = useT();
  const [title, setTitle] = useState(initial?.title ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [slot, setSlot] = useState(initial?.slot ?? "dinner");
  const [timeMin, setTimeMin] = useState(String(initial?.timeMin ?? 30));
  const [batch, setBatch] = useState(initial?.batch ?? false);
  const [image, setImage] = useState(initial?.image ?? "");
  const [eq, setEq] = useState<string[]>(initial?.equipment ?? ["stove"]);
  const [tags, setTagList] = useState<string[]>(initial?.tags ?? []);
  const [tagInput, setTagInput] = useState("");
  const [rows, setRows] = useState<Row[]>(initial?.ingredients?.map((i) => ({ ingredientId: i.ingredientId, amount: String(i.amount) })) ?? []);
  const [stepsText, setStepsText] = useState(initial?.steps.join("\n") ?? "");
  const [base, setBase] = useState<IngredientRef[]>([]);
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const titleRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    api.ingredients().then(setBase).catch(() => setBase([]));
    titleRef.current?.focus();
  }, []);
  // «Расписать подробнее»: тот же помощник, что и у своих рецептов; название базы не трогаем
  const [aiBusy, setAiBusy] = useState(false);
  const [before, setBefore] = useState<{ description: string; stepsText: string } | null>(null);
  const detail = async () => {
    setError(null);
    setAiBusy(true);
    try {
      const out = await api.assistRecipe({ action: "improve", lang: "ru", title: title.trim(), description: description.trim(), steps: stepsText.split(/\r?\n/).map((s) => s.trim()).filter(Boolean) });
      setBefore({ description, stepsText });
      setDescription(out.description);
      setStepsText(out.steps.join("\n"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setAiBusy(false);
    }
  };
  const byId = useMemo(() => new Map(base.map((i) => [i.id, i])), [base]);
  const suggestions = useMemo(() => {
    const s = query.trim().toLowerCase();
    if (s.length < 2) return [];
    const taken = new Set(rows.map((r) => r.ingredientId));
    return base.filter((i) => (i.label.toLowerCase().includes(s) || i.id.includes(s)) && !taken.has(i.id)).slice(0, 8);
  }, [query, base, rows]);
  const toggle = (list: string[], id: string, set: (v: string[]) => void) => set(list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);
  const addTag = (v: string) => {
    const tg = v.trim().toLowerCase();
    if (tg && !tags.includes(tg)) setTagList([...tags, tg]);
    setTagInput("");
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    const body: CatalogRecipeInput = {
      id: initial?.id ?? "",
      title: title.trim(),
      description: description.trim(),
      slot,
      timeMin: Math.round(Number(timeMin) || 0),
      batch,
      image,
      equipment: eq,
      tags,
      steps: stepsText.split(/\r?\n/).map((s) => s.trim()).filter(Boolean),
      ingredients: rows.map((r) => ({ ingredientId: r.ingredientId, amount: Number(String(r.amount).replace(",", ".")) || 0 })),
    };
    setSaving(true);
    try {
      onSaved(await api.adminSaveRecipe(body));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <form className="ownform" onSubmit={submit} aria-label={initial ? t("admin.recipe.edit") : t("admin.recipe.new")}>
      <h2 className="ownform__title">
        {initial ? t("admin.recipe.edit") : t("admin.recipe.new")}
        {initial && <small className="ownform__id"> · {initial.id}</small>}
      </h2>
      <label className="auth__field">
        <span>{t("own.title")}</span>
        <input ref={titleRef} className="form-control" value={title} onChange={(e) => setTitle(e.target.value)} maxLength={80} required />
      </label>
      {photos && (
        <div className="auth__field">
          <span>{t("own.photo")}</span>
          <PhotoField value={image} onChange={(url) => setImage(url)} kind="recipe" />
        </div>
      )}
      <label className="auth__field">
        <span>{t("own.description")}</span>
        <textarea className="form-control" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} maxLength={300} />
      </label>
      <div className="ownform__row">
        <div className="auth__field">
          <span>{t("own.slot")}</span>
          <Select aria-label={t("own.slot")} value={slot} onChange={setSlot} options={["breakfast", "lunch", "dinner", "snack"].map((s) => ({ value: s, label: slotLabel(lang, s) }))} />
        </div>
        <label className="auth__field ownform__time">
          <span>{t("own.time")}</span>
          <input className="form-control" inputMode="numeric" value={timeMin} onChange={(e) => setTimeMin(e.target.value)} required />
        </label>
      </div>
      <button type="button" className="switch" role="switch" aria-checked={batch} onClick={() => setBatch(!batch)}>
        <span className="switch__text">{t("admin.recipe.batch")}</span>
        <span className="switch__track" aria-hidden>
          <span className="switch__knob" />
        </span>
      </button>
      <div className="auth__field">
        <span>{t("own.equipment")}</span>
        <div className="chips">
          {equipment.map((e) => (
            <button key={e.id} type="button" className="chip chip--sm" aria-pressed={eq.includes(e.id)} onClick={() => toggle(eq, e.id, setEq)}>
              {e.label}
            </button>
          ))}
        </div>
      </div>
      <div className="auth__field">
        <span>{t("own.tags")}</span>
        <div className="chips">
          {tags.map((tg) => (
            <button key={tg} type="button" className="chip chip--sm chip--remove" onClick={() => setTagList(tags.filter((x) => x !== tg))} aria-label={`${t("delete")} ${tg}`}>
              {tg} <X size={12} aria-hidden />
            </button>
          ))}
        </div>
        <input className="form-control" list="admin-tags" value={tagInput} onChange={(e) => setTagInput(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter" || e.key === ",") { e.preventDefault(); addTag(tagInput); } }} onBlur={() => tagInput && addTag(tagInput)} placeholder={t("admin.recipe.tag.ph")} />
        <datalist id="admin-tags">
          {knownTags.map((tg) => (
            <option key={tg} value={tg} />
          ))}
        </datalist>
      </div>
      <div className="auth__field">
        <span>{t("own.ingredients")}</span>
        <ul className="ownform__ings">
          {rows.map((r) => {
            const ing = byId.get(r.ingredientId);
            return (
              <li key={r.ingredientId} className="ownform__ing">
                <span className="ownform__ing-name">{ing?.label ?? r.ingredientId}</span>
                <label className="ownform__amount">
                  <input className="form-control" inputMode="decimal" value={r.amount} onChange={(e) => setRows(rows.map((x) => (x.ingredientId === r.ingredientId ? { ...x, amount: e.target.value } : x)))} aria-label={ing?.label ?? r.ingredientId} />
                  <span className="ownform__unit">{ing?.unit === "pcs" ? t("unit.pcs") : ing?.unit === "ml" ? t("unit.ml") : t("unit.g")}</span>
                </label>
                <button type="button" className="planrow__del" onClick={() => setRows(rows.filter((x) => x.ingredientId !== r.ingredientId))} aria-label={t("delete")}>
                  <X size={16} aria-hidden />
                </button>
              </li>
            );
          })}
        </ul>
        <div className="search">
          <Search size={18} aria-hidden />
          <input className="form-control" value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t("own.ingredients.search")} aria-label={t("own.ingredients")} autoComplete="off" />
        </div>
        {suggestions.length > 0 && (
          <div className="suggest" role="listbox">
            {suggestions.map((s) => (
              <button key={s.id} type="button" role="option" aria-selected={false} onClick={() => { setRows([...rows, { ingredientId: s.id, amount: s.unit === "pcs" ? "1" : s.pantry ? "5" : "100" }]); setQuery(""); }}>
                {s.label} <small>{s.id}</small>
              </button>
            ))}
          </div>
        )}
      </div>
      <label className="auth__field">
        <span>{t("own.steps")}</span>
        <textarea className="form-control" rows={10} value={stepsText} onChange={(e) => setStepsText(e.target.value)} required />
        <small>{t("admin.recipe.steps.hint")}</small>
      </label>
      {ai && (
        <div className="ownform__ai" aria-busy={aiBusy}>
          <div className="ownform__ai-row">
            <button type="button" className="btn btn-soft btn-sm" disabled={aiBusy} onClick={detail}>
              <Sparkles size={16} aria-hidden /> {t("admin.recipe.detail")}
            </button>
            {before && (
              <button type="button" className="btn btn-link btn-sm" onClick={() => { setDescription(before.description); setStepsText(before.stepsText); setBefore(null); }}>
                <Undo2 size={16} aria-hidden /> {t("own.ai.undo")}
              </button>
            )}
          </div>
          <small role="status">{aiBusy ? t("own.ai.working") : t("admin.recipe.detail.hint")}</small>
        </div>
      )}
      {error && (
        <p className="ownform__error" role="alert">
          {error}
        </p>
      )}
      <div className="ownform__actions">
        <button type="submit" className="btn btn-primary" disabled={saving}>
          {t("save")}
        </button>
        <button type="button" className="btn btn-soft" onClick={onCancel}>
          {t("cancel")}
        </button>
      </div>
    </form>
  );
}
