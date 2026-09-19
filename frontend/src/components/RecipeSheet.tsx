import { useEffect, useState } from "react";
import { ArrowLeftRight, ChefHat, Clock, Flame, Undo2, Users } from "lucide-react";
import { CookMode } from "./CookMode";
import { CollectionButton } from "./CollectionPicker";
import { Sheet } from "./Sheet";
import { PartnerRow } from "./PartnerRow";
import { SocialBar } from "./SocialBar";
import { api } from "../lib/api";
import type { Recipe, SubOption, SubRow } from "../lib/types";
import { qty } from "../lib/format";
import { intlLocale, useT } from "../i18n";
import { IngredientPic } from "./IngredientPic";

export function RecipeSheet({ recipeId, portions, country, onClose }: { recipeId: string | null; portions: number; country?: string; onClose: () => void }) {
  const { t, tn, lang } = useT();
  const [recipe, setRecipe] = useState<Recipe | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [cooking, setCooking] = useState(false);
  // замены продуктов: варианты с сервера, выбранные — в состоянии (без сохранения)
  const [subs, setSubs] = useState<SubRow[]>([]);
  const [openSub, setOpenSub] = useState<string | null>(null);
  const [swapped, setSwapped] = useState<Record<string, SubOption>>({});
  useEffect(() => {
    if (!recipeId) return;
    setSubs([]);
    setSwapped({});
    setOpenSub(null);
    api.recipeSubs(recipeId, country).then(setSubs).catch(() => setSubs([]));
  }, [recipeId, lang, country]);
  const subsFor = (id: string) => subs.find((r) => r.ingredientId === id)?.options ?? [];
  useEffect(() => {
    if (!toast) return;
    const id = window.setTimeout(() => setToast(null), 2000);
    return () => window.clearTimeout(id);
  }, [toast]);

  useEffect(() => {
    if (!recipeId) return;
    let alive = true;
    setRecipe(null);
    setError(null);
    api
      .recipe(recipeId)
      .then((r) => alive && setRecipe(r))
      .catch((e: Error) => alive && setError(e.message));
    return () => {
      alive = false;
    };
  }, [recipeId, lang]);

  const isKid = recipe?.tags.includes("kidmenu") ?? false;
  const p = isKid ? 1 : Math.max(1, Math.round(portions * 2) / 2);

  return (
    <Sheet open={recipeId !== null} onClose={onClose} title={recipe?.title ?? t("recipe")} closeLabel={t("close")}>
      {error && <p className="error-inline">{error}</p>}
      {!recipe && !error && (
        <div style={{ display: "grid", gap: 10, paddingTop: 12 }} aria-busy="true" aria-label={t("recipe.loading")}>
          <div className="skeleton" style={{ height: 26, width: "70%" }} />
          <div className="skeleton" style={{ height: 14, width: "45%" }} />
          <div className="skeleton" style={{ height: 120 }} />
        </div>
      )}
      {recipe && (
        <>
          {recipe.image && <img className="bsheet__img" src={recipe.image} alt="" width={768} height={512} loading="lazy" />}
          <h2>
            {recipe.title}
            {recipe.own && <span className="dish__own">{t("own.badge")}</span>}
          </h2>
          <div className="bsheet__meta">
            <span>
              <Clock size={14} aria-hidden /> {recipe.timeMin} {t("min")}
            </span>
            <span>
              <Flame size={14} aria-hidden /> <span className="num">{Math.round(recipe.kcal)}</span> {t("recipe.perPortion")}
            </span>
            <span>
              <Users size={14} aria-hidden /> {isKid ? t("recipe.kidPortion") : t("recipe.for", { n: p.toLocaleString(intlLocale(lang)), portions: tn("portions", Number.isInteger(p) ? p : 5) })}
            </span>
          </div>
          {recipe.author && <p className="social__by">{t("recipe.by", { nick: recipe.author })}</p>}
          {recipe.description && <p className="bsheet__lead">{recipe.description}</p>}
          <SocialBar recipeId={recipe.id} initial={recipe.stats} onToast={setToast} extra={<CollectionButton recipeId={recipe.id} onToast={setToast} />} />
          {recipe.steps.length > 0 && (
            <button type="button" className="btn btn-primary cook-btn" onClick={() => setCooking(true)}>
              <ChefHat size={18} aria-hidden /> {t("cook.open")}
            </button>
          )}
          {cooking && <CookMode recipe={recipe} portions={portions} onClose={() => setCooking(false)} />}
          {toast && (
            <div className="toast toast--sheet" role="status">
              {toast}
            </div>
          )}
          <h3>{t("recipe.products")}</h3>
          <div>
            {recipe.ingredients.map((i) => {
              const sw = swapped[i.ingredientId];
              const opts = subsFor(i.ingredientId);
              return (
                <div key={i.ingredientId} className={"ing" + (i.pantry ? " ing--pantry" : "") + (sw ? " ing--swapped" : "")}>
                  <span>
                    {sw ? sw.name : i.name}
                    <IngredientPic src={sw ? undefined : i.image} />
                    {opts.length > 0 && !sw && (
                      <button type="button" className="ings__swap" onClick={() => setOpenSub(openSub === i.ingredientId ? null : i.ingredientId)} aria-expanded={openSub === i.ingredientId} aria-label={t("subs.button")} title={t("subs.button")}>
                        <ArrowLeftRight size={13} aria-hidden />
                      </button>
                    )}
                    {sw && (
                      <button type="button" className="subs__back" onClick={() => setSwapped(({ [i.ingredientId]: _x, ...rest }) => rest)} aria-label={t("subs.back")}>
                        <Undo2 size={13} aria-hidden /> {t("subs.back")}
                      </button>
                    )}
                  </span>
                  <span className="num">{qty((sw ? sw.amount : i.amount) * p, sw ? sw.unit : i.unit, lang)}</span>
                  {openSub === i.ingredientId && (
                    <div className="subs__pop" role="menu">
                      <div className="subs__title">{t("subs.title")}</div>
                      {opts.map((o) => (
                        <button key={o.id} type="button" role="menuitem" className="subs__opt" onClick={() => { setSwapped({ ...swapped, [i.ingredientId]: o }); setOpenSub(null); }}>
                          <b>{o.name}</b>
                          <span className="num">{qty(o.amount * p, o.unit, lang)}</span>
                          {o.note && <small>{o.note}</small>}
                          {(o.kcalDelta !== 0 || o.costDelta !== 0) && (
                            <small className="subs__delta">
                              {o.kcalDelta !== 0 ? `${o.kcalDelta > 0 ? "+" : "−"}${Math.abs(o.kcalDelta)} ${t("kcal")}` : ""}
                              {o.kcalDelta !== 0 && o.costDelta !== 0 ? " · " : ""}
                              {o.costDelta !== 0 ? `${o.costDelta > 0 ? "+" : "−"}${Math.abs(o.costDelta).toLocaleString(intlLocale(lang))}${o.symbol ? ` ${o.symbol}` : ""}` : ""}
                            </small>
                          )}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
          <h3>{t("recipe.steps")}</h3>
          <ol>
            {recipe.steps.map((s, n) => (
              <li key={n}>{s}</li>
            ))}
          </ol>
          <PartnerRow equipment={recipe.equipment} country={country} onToast={setToast} />
        </>
      )}
    </Sheet>
  );
}
