import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ChevronDown, FolderOpen, Link2, Pencil, Plus, Trash2, X } from "lucide-react";
import { EmptyState } from "./EmptyState";
import { api } from "../lib/api";
import type { Collection, Favorite } from "../lib/types";
import { slotLabel } from "../lib/types";
import { useConfirm } from "./Confirm";
import { useT } from "../i18n";

// Коллекции в кабинете: папки с числом рецептов; раскрытие показывает рецепты (убрать можно тут же),
// «Собрать неделю из коллекции» ведёт в квиз с ?collection=id.

export function CollectionsPanel({ onToast }: { onToast: (m: string) => void }) {
  const { t, lang } = useT();
  const confirm = useConfirm();
  const [list, setList] = useState<Collection[] | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [items, setItems] = useState<Record<string, Favorite[]>>({});
  const [name, setName] = useState("");
  const [renaming, setRenaming] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const recipeHref = (id: string) => `${lang === "ru" ? "" : `/${lang}`}/recipe/${id}`;

  useEffect(() => {
    api.collections().then(setList).catch(() => setList([]));
  }, []);
  const toggleOpen = (c: Collection) => {
    const next = open === c.id ? null : c.id;
    setOpen(next);
    if (next && !items[c.id]) api.collectionItems(c.id).then((x) => setItems((m) => ({ ...m, [c.id]: x }))).catch(() => {});
  };
  const create = async () => {
    if (!name.trim()) return;
    try {
      const c = await api.collectionCreate(name.trim());
      setList([...(list ?? []), c]);
      setName("");
    } catch (e) {
      onToast((e as Error).message);
    }
  };
  const remove = async (c: Collection) => {
    if (!(await confirm({ title: t("coll.delete.confirm", { name: c.name }), text: t("coll.delete.text"), ok: t("delete"), danger: true }))) return;
    try {
      await api.collectionDelete(c.id);
      setList((list ?? []).filter((x) => x.id !== c.id));
    } catch (e) {
      onToast((e as Error).message);
    }
  };
  const rename = async (c: Collection) => {
    const n = newName.trim();
    setRenaming(null);
    if (!n || n === c.name) return;
    try {
      await api.collectionRename(c.id, n);
      setList((list ?? []).map((x) => (x.id === c.id ? { ...x, name: n } : x)));
    } catch (e) {
      onToast((e as Error).message);
    }
  };
  const removeItem = async (c: Collection, rid: string) => {
    try {
      await api.collectionToggle(c.id, rid, false);
      setItems((m) => ({ ...m, [c.id]: (m[c.id] ?? []).filter((x) => x.id !== rid) }));
      setList((list ?? []).map((x) => (x.id === c.id ? { ...x, recipes: x.recipes.filter((r) => r !== rid) } : x)));
    } catch (e) {
      onToast((e as Error).message);
    }
  };

  return (
    <>
      <h3 className="account__sub">{t("coll.title")}</h3>
      <p className="quiz__hint">{t("coll.hint")}</p>
      {list === null && <div className="skeleton" style={{ height: 60 }} />}
      {list?.length === 0 && <EmptyState icon={<FolderOpen size={20} />} text={t("coll.empty")} />}
      <div className="coll">
        {list?.map((c) => (
          <div key={c.id}>
            <div className="planrow">
              {renaming === c.id ? (
                <form className="planrow__main coll__actions" onSubmit={(e) => { e.preventDefault(); void rename(c); }}>
                  <input className="form-control" value={newName} onChange={(e) => setNewName(e.target.value)} maxLength={40} autoFocus aria-label={t("coll.rename")} />
                  <button type="submit" className="btn btn-soft btn-sm">{t("save")}</button>
                </form>
              ) : (
                <button type="button" className="planrow__main planrow__main--btn" onClick={() => toggleOpen(c)} aria-expanded={open === c.id}>
                  <span className="planrow__title">
                    <FolderOpen size={16} aria-hidden style={{ verticalAlign: -3, marginRight: 6 }} />
                    {c.name}
                  </span>
                  <span className="planrow__meta">{c.recipes.length} {t("recipes.n")}</span>
                </button>
              )}
              <span className="coll__actions">
                <Link to={`/?s=1&collection=${c.id}`} className="btn btn-soft btn-sm" title={t("coll.week")} aria-label={t("coll.week")}>
                  {t("coll.week.short")}
                </Link>
                <button
                  type="button"
                  className={"planrow__del" + (c.public ? " planrow__share" : "")}
                  onClick={async () => {
                    try {
                      const upd = await api.collectionPublish(c.id, !c.public);
                      setList((list ?? []).map((x) => (x.id === c.id ? { ...x, ...upd } : x)));
                      if (upd.public) {
                        await navigator.clipboard.writeText(`${window.location.origin}/collection/${upd.slug}`).catch(() => {});
                        onToast(t("coll.shared"));
                      } else onToast(t("coll.closed"));
                    } catch (e) {
                      onToast((e as Error).message);
                    }
                  }}
                  aria-label={c.public ? t("coll.unshare") : t("coll.share")}
                  title={c.public ? t("coll.unshare") : t("coll.share")}
                >
                  <Link2 size={16} aria-hidden />
                </button>
                <button type="button" className="planrow__del" onClick={() => { setRenaming(c.id); setNewName(c.name); }} aria-label={t("coll.rename")} title={t("coll.rename")}>
                  <Pencil size={15} aria-hidden />
                </button>
                <button type="button" className="planrow__del" onClick={() => remove(c)} aria-label={t("coll.delete")} title={t("coll.delete")}>
                  <Trash2 size={16} aria-hidden />
                </button>
                <button type="button" className="planrow__del" onClick={() => toggleOpen(c)} aria-label={t("purch.more", { n: c.recipes.length })} aria-expanded={open === c.id}>
                  <ChevronDown size={16} aria-hidden style={{ transform: open === c.id ? "rotate(180deg)" : undefined }} />
                </button>
              </span>
            </div>
            {open === c.id && (
              <div className="coll__items">
                {!items[c.id] && <div className="skeleton" style={{ height: 40 }} />}
                {items[c.id]?.length === 0 && <p className="ownform__empty">{t("coll.items.empty")}</p>}
                {items[c.id]?.map((f) => (
                  <div className="planrow" key={f.id}>
                    <a className="planrow__main" href={recipeHref(f.id)}>
                      <span className="planrow__title">
                        {f.title}
                        {f.own && <span className="dish__own">{t("own.badge")}</span>}
                      </span>
                      <span className="planrow__meta">{slotLabel(lang, f.slot)}</span>
                    </a>
                    <button type="button" className="planrow__del" onClick={() => removeItem(c, f.id)} aria-label={t("coll.remove.item")} title={t("coll.remove.item")}>
                      <X size={16} aria-hidden />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
      <form className="coll__new" onSubmit={(e) => { e.preventDefault(); void create(); }}>
        <input className="form-control" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("coll.new.ph")} maxLength={40} aria-label={t("coll.new")} />
        <button type="submit" className="btn btn-soft" disabled={!name.trim()}>
          <Plus size={16} aria-hidden /> {t("coll.create")}
        </button>
      </form>
    </>
  );
}
