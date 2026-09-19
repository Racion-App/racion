import { useEffect, useState } from "react";
import { Check, FolderPlus, Plus } from "lucide-react";
import { Sheet } from "./Sheet";
import { api, ApiError } from "../lib/api";
import type { Collection } from "../lib/types";
import { useAuth } from "../lib/auth";
import { useT } from "../i18n";

// «В коллекцию»: кнопка у рецепта и лист со списком коллекций — отметка добавляет/убирает,
// внизу поле для новой. Гостю кнопка не показывается.

export function CollectionButton({ recipeId, onToast, className }: { recipeId: string; onToast: (m: string) => void; className?: string }) {
  const { t } = useT();
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const [list, setList] = useState<Collection[] | null>(null);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open && list === null) api.collections().then(setList).catch(() => setList([]));
  }, [open, list]);
  if (!user) return null;
  const inCount = list?.filter((c) => c.recipes.includes(recipeId)).length ?? 0;

  const toggle = async (c: Collection) => {
    const on = !c.recipes.includes(recipeId);
    try {
      await api.collectionToggle(c.id, recipeId, on);
      setList((list ?? []).map((x) => (x.id === c.id ? { ...x, recipes: on ? [...x.recipes, recipeId] : x.recipes.filter((r) => r !== recipeId) } : x)));
      onToast(t(on ? "coll.added" : "coll.removed", { name: c.name }));
    } catch (e) {
      onToast(e instanceof ApiError ? e.message : String(e));
    }
  };
  const create = async () => {
    if (!name.trim() || busy) return;
    setBusy(true);
    try {
      const c = await api.collectionCreate(name.trim());
      setList([...(list ?? []), c]);
      setName("");
      await toggle(c);
    } catch (e) {
      onToast(e instanceof ApiError ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <button type="button" className={className ?? "social__btn"} aria-pressed={inCount > 0} onClick={() => setOpen(true)} aria-label={t("coll.add")} title={t("coll.add")}>
        <FolderPlus size={16} aria-hidden /> {inCount > 0 ? <span className="num">{inCount}</span> : t("coll.add")}
      </button>
      <Sheet open={open} onClose={() => setOpen(false)} title={t("coll.pick")} closeLabel={t("close")}>
        <h2>{t("coll.pick")}</h2>
        {list === null && <div className="skeleton" style={{ height: 60 }} />}
        {list?.length === 0 && <p className="bsheet__lead">{t("coll.empty")}</p>}
        <ul className="langlist" role="listbox" aria-label={t("coll.pick")}>
          {list?.map((c) => {
            const on = c.recipes.includes(recipeId);
            return (
              <li key={c.id}>
                <button type="button" role="option" aria-selected={on} className={"langlist__item" + (on ? " is-on" : "")} onClick={() => toggle(c)}>
                  <span className="langlist__name">
                    {c.name}
                    <small>{c.recipes.length}</small>
                  </span>
                  {on && <Check size={18} aria-hidden />}
                </button>
              </li>
            );
          })}
        </ul>
        <form
          className="coll__new"
          onSubmit={(e) => {
            e.preventDefault();
            void create();
          }}
        >
          <input className="form-control" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("coll.new.ph")} maxLength={40} aria-label={t("coll.new")} />
          <button type="submit" className="btn btn-soft" disabled={!name.trim() || busy}>
            <Plus size={16} aria-hidden /> {t("coll.create")}
          </button>
        </form>
      </Sheet>
    </>
  );
}
