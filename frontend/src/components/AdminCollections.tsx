import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Eye, EyeOff, Plus, Search, Trash2, X } from "lucide-react";
import { api, ApiError } from "../lib/api";
import type { AdminRecipe, Collection } from "../lib/types";
import { AList, ARow } from "./AdminList";
import { PhotoField } from "./PhotoField";
import { useConfirm } from "./Confirm";
import { locales, useT } from "../i18n";

// Редакционные подборки: «Стол на Новый год», «Быстрые ужины». Список → форма: название, адрес,
// описание, обложка, публичность, рецепты (поиск по базе, порядок как добавили).

export function AdminCollections({ photos, onToast }: { photos: boolean; onToast: (s: string) => void }) {
  const { t } = useT();
  const confirm = useConfirm();
  const [list, setList] = useState<Collection[] | null>(null);
  const [editing, setEditing] = useState<Collection | "new" | null>(null);
  const load = () => api.adminCollections().then(setList).catch((e: Error) => onToast(e.message));
  useEffect(() => {
    void load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (editing) {
    return (
      <CollectionForm
        initial={editing === "new" ? null : editing}
        photos={photos}
        onSaved={(c) => { setEditing(null); onToast(t("admin.coll.saved")); void load(); window.open(`/collection/${c.slug}`, "_blank", "noopener"); }}
        onCancel={() => setEditing(null)}
      />
    );
  }
  return (
    <section className="admin__section">
      <div className="admin__row">
        <p className="admin__hint">{t("coll.public.title")}</p>
        <button type="button" className="btn btn-primary" onClick={() => setEditing("new")}>
          <Plus size={16} aria-hidden /> {t("admin.coll.new")}
        </button>
      </div>
      <AList empty={list?.length === 0 && <p className="state__box">{t("coll.empty")}</p>}>
        {list?.map((c) => (
          <ARow
            key={c.id}
            media={c.coverAuto ? <img className="alist__thumb" src={c.coverAuto} alt="" /> : <span className="alist__thumb alist__thumb--dinner" aria-hidden />}
            title={
              <>
                {c.name}
                {!c.public && <span className="badge badge--moderator">{t("admin.coll.draft")}</span>}
              </>
            }
            meta={`${c.recipes.length} ${t("recipes.n")} · /collection/${c.slug}`}
            onClick={() => setEditing(c)}
            right={
              c.public ? (
                <a className="alist__id" href={`/collection/${c.slug}`} target="_blank" rel="noopener">
                  <Eye size={16} aria-hidden />
                </a>
              ) : (
                <span className="alist__id"><EyeOff size={16} aria-hidden /></span>
              )
            }
            actions={
              <button
                type="button"
                className="planrow__del"
                aria-label={t("delete")}
                onClick={async () => {
                  if (!(await confirm({ title: t("admin.coll.confirm", { name: c.name }), text: t("coll.delete.text"), ok: t("delete"), danger: true }))) return;
                  try {
                    await api.adminDeleteCollection(c.id);
                    void load();
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
    </section>
  );
}

function CollectionForm({ initial, photos, onSaved, onCancel }: { initial: Collection | null; photos: boolean; onSaved: (c: Collection) => void; onCancel: () => void }) {
  const { t } = useT();
  const [name, setName] = useState(initial?.name ?? "");
  const [slug, setSlug] = useState(initial?.slug ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [cover, setCover] = useState(initial?.cover ?? "");
  const [isPublic, setPublic] = useState(initial?.public ?? false);
  const [recipes, setRecipes] = useState<string[]>(initial?.recipes ?? []);
  const [names, setNames] = useState<Record<string, string>>(initial?.names ?? {});
  const [descriptions, setDescriptions] = useState<Record<string, string>>(initial?.descriptions ?? {});
  const [titles, setTitles] = useState<Record<string, AdminRecipe>>({});
  const [q, setQ] = useState("");
  const [found, setFound] = useState<AdminRecipe[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    // названия уже добавленных: тянем по поиску пустой строкой не выйдет — просим каждую по id
    recipes.forEach((id) => {
      if (!titles[id]) api.adminRecipe(id).then((r) => setTitles((m) => ({ ...m, [id]: r }))).catch(() => {});
    });
  }, [recipes]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (q.trim().length < 2) {
      setFound([]);
      return;
    }
    const id = window.setTimeout(() => api.adminRecipes(q).then((r) => setFound(r.items.filter((x) => !recipes.includes(x.id)).slice(0, 8))).catch(() => setFound([])), 200);
    return () => window.clearTimeout(id);
  }, [q, recipes]);
  const known = useMemo(() => titles, [titles]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    try {
      onSaved(await api.adminSaveCollection({ id: initial?.id ?? "", name: name.trim(), slug: slug.trim(), description: description.trim(), cover, public: isPublic, recipes, names, descriptions }));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <form className="ownform" onSubmit={submit}>
      <h2 className="ownform__title">{initial ? t("admin.coll.edit") : t("admin.coll.new")}</h2>
      <label className="auth__field">
        <span>{t("own.title")}</span>
        <input className="form-control" value={name} onChange={(e) => setName(e.target.value)} maxLength={40} required autoFocus />
      </label>
      <label className="auth__field">
        <span>{t("admin.coll.slug")}</span>
        <input className="form-control" value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="new-year-table" pattern="[a-z0-9_-]*" />
      </label>
      <label className="auth__field">
        <span>{t("admin.coll.desc")}</span>
        <textarea className="form-control" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} maxLength={600} />
      </label>
      {photos && (
        <div className="auth__field">
          <span>{t("admin.coll.cover")}</span>
          <PhotoField value={cover} onChange={(url) => setCover(url)} kind="recipe" />
        </div>
      )}
      <details className="admin__i18n">
        <summary>{t("admin.coll.i18n")} <small>{Object.keys(names).filter((k) => names[k]).length}/{locales().filter((l) => l.code !== "ru").length}</small></summary>
        <p className="admin__hint">{t("admin.coll.i18n.hint")}</p>
        {locales().filter((l) => l.code !== "ru").map((l) => (
          <div key={l.code} className="admin__i18n-row">
            <span className={`fi fi-${l.flag}`} aria-hidden /> <b>{l.name}</b>
            <input className="form-control" value={names[l.code] ?? ""} onChange={(e) => setNames({ ...names, [l.code]: e.target.value })} maxLength={40} placeholder={name} aria-label={`${t("own.title")} · ${l.name}`} />
            <textarea className="form-control" rows={2} value={descriptions[l.code] ?? ""} onChange={(e) => setDescriptions({ ...descriptions, [l.code]: e.target.value })} maxLength={600} placeholder={description} aria-label={`${t("admin.coll.desc")} · ${l.name}`} />
          </div>
        ))}
      </details>
      <button type="button" className="switch" role="switch" aria-checked={isPublic} onClick={() => setPublic(!isPublic)}>
        <span className="switch__text">{t("admin.coll.public")}</span>
        <span className="switch__track" aria-hidden>
          <span className="switch__knob" />
        </span>
      </button>
      <div className="auth__field">
        <span>{t("admin.coll.recipes")}</span>
        <ul className="ownform__ings">
          {recipes.map((id) => (
            <li key={id} className="ownform__ing" style={{ gridTemplateColumns: "minmax(0, 1fr) 36px" }}>
              <span className="ownform__ing-name">{known[id]?.title ?? id}</span>
              <button type="button" className="planrow__del" onClick={() => setRecipes(recipes.filter((x) => x !== id))} aria-label={t("delete")}>
                <X size={16} aria-hidden />
              </button>
            </li>
          ))}
        </ul>
        <div className="search">
          <Search size={18} aria-hidden />
          <input className="form-control" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t("admin.recipe.search")} aria-label={t("admin.coll.add")} autoComplete="off" />
        </div>
        {found.length > 0 && (
          <div className="suggest" role="listbox">
            {found.map((r) => (
              <button key={r.id} type="button" role="option" aria-selected={false} onClick={() => { setRecipes([...recipes, r.id]); setTitles((m) => ({ ...m, [r.id]: r })); setQ(""); }}>
                {r.title} <small>{r.id}</small>
              </button>
            ))}
          </div>
        )}
      </div>
      {error && (
        <p className="ownform__error" role="alert">
          {error}
        </p>
      )}
      <div className="ownform__actions">
        <button type="submit" className="btn btn-primary" disabled={saving || !name.trim()}>
          {t("save")}
        </button>
        <button type="button" className="btn btn-soft" onClick={onCancel}>
          {t("cancel")}
        </button>
      </div>
    </form>
  );
}
