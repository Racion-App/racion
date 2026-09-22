import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ArrowLeft, Minus, Plus, Trash2, Users, X } from "lucide-react";
import { SiteFooter } from "../components/SiteFooter";
import { TopBar } from "../components/TopBar";
import { Select } from "../components/Select";
import { StoreMark } from "../components/StoreMark";
import { api } from "../lib/api";
import { track } from "../lib/analytics";
import { readDraft } from "../lib/draft";
import { clearTable, readTable, setServings, toggleTable, type TableItem } from "../lib/table";
import { approx } from "../lib/format";
import type { Meta, Params, Recipe } from "../lib/types";
import { langCountry, useLang, useT } from "../i18n";

// «Мой стол»: блюда человек набирает сам на страницах каталога и подборок, здесь задаёт порции
// и получает обычный план — с ценами, списком покупок и округлением до упаковок.
// Порции у каждого блюда свои: три салата на всю компанию, курица на четверых.

export function Table() {
  const nav = useNavigate();
  const { t, tn, lang } = useT();
  const { ready } = useLang();
  const [items, setItems] = useState<TableItem[]>(() => readTable());
  const [recipes, setRecipes] = useState<Record<string, Recipe>>({});
  const [draft] = useState<Partial<Params>>(() => readDraft());
  const [country, setCountry] = useState(draft.country || langCountry(lang));
  const [store, setStore] = useState(draft.store || "");
  const [guests, setGuests] = useState(draft.adults && draft.adults > 0 ? draft.adults : 4);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.meta(country).then((m) => {
      setMeta(m);
      const own = m.stores.filter((s) => s.country === country);
      if (!own.some((s) => s.code === store)) setStore(own[0]?.code ?? "");
    }).catch(() => setMeta(null));
    setRecipes({}); // цены рецептов зависят от страны: при смене перезапрашиваем
  }, [country]); // eslint-disable-line react-hooks/exhaustive-deps

  // карточки блюд подтягиваем по одной: стол маленький, отдельная ручка ради него не нужна
  useEffect(() => {
    let alive = true;
    const missing = items.map((x) => x.id).filter((id) => !recipes[id]);
    if (missing.length === 0) return;
    Promise.all(missing.map((id) => api.recipe(id, country).catch(() => null))).then((list) => {
      if (!alive) return;
      const add: Record<string, Recipe> = {};
      list.forEach((r) => { if (r) add[r.id] = r; });
      setRecipes((prev) => ({ ...prev, ...add }));
    });
    return () => { alive = false; };
  }, [items, country]); // eslint-disable-line react-hooks/exhaustive-deps

  const stores = useMemo(() => (meta?.stores ?? []).filter((s) => s.country === country), [meta, country]);
  const cy = useMemo(() => meta?.countries.find((c) => c.code === country), [meta, country]);
  // грубая прикидка до сборки: цена порции × порции. Точную даёт сервер, он же округляет до упаковок.
  const rough = useMemo(
    () => items.reduce((sum, x) => sum + (recipes[x.id]?.cost ?? 0) * (x.servings || guests), 0),
    [items, recipes, guests],
  );

  if (!ready) return null;

  const build = async () => {
    setBusy(true);
    setError(null);
    try {
      const equipment = draft.equipment?.length ? draft.equipment : ["stove", "oven", "microwave"];
      const plan = await api.createTable(
        items.map((x) => ({ recipeId: x.id, servings: x.servings || guests })),
        guests,
        { ...draft, equipment, country, store, region: draft.region ?? "" } as Params,
      );
      track("table_built", { dishes: items.length, guests });
      nav(`/plan/${plan.id}`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell">
      <TopBar />
      <main className="occ">
        <Link to="/" className="occ__back">
          <ArrowLeft size={16} aria-hidden /> {t("home")}
        </Link>
        <div className="occ__head">
          <span className="occ__icon" aria-hidden><Users size={26} /></span>
          <div>
            <h1 className="occ__title">{t("table.title")}</h1>
            <p className="occ__lead">{t("table.lead")}</p>
          </div>
        </div>

        {items.length === 0 ? (
          <div className="state__box">
            <p>{t("table.empty")}</p>
            <Link className="btn btn-primary" to="/collections">{t("table.empty.go")}</Link>
          </div>
        ) : (
          <>
            <p className="tablelist__hint">{t("table.portions.hint")}</p>
            <ul className="tablelist">
              {items.map((x) => {
                const r = recipes[x.id];
                const n = x.servings || guests;
                return (
                  <li key={x.id} className="tablelist__row">
                    {r?.image
                      ? <img className="tablelist__img" src={r.image} alt="" loading="lazy" width={56} height={56} />
                      : <span className="tablelist__img tablelist__img--empty" aria-hidden />}
                    <div className="tablelist__body">
                      <Link className="tablelist__title" to={`/recipe/${x.id}`}>{r?.title ?? x.id}</Link>
                      {r && <span className="tablelist__meta num">{approx((r.cost ?? 0) * n, cy, lang)}</span>}
                    </div>
                    <div className="tablelist__portions" role="group" aria-label={t("table.portions")}>
                      <button type="button" className="tablelist__step" onClick={() => setItems(setServings(x.id, Math.max(1, n - 1)))} aria-label={t("table.less")} disabled={n <= 1}>
                        <Minus size={15} aria-hidden />
                      </button>
                      <output className="tablelist__n num">{n}</output>
                      <button type="button" className="tablelist__step" onClick={() => setItems(setServings(x.id, Math.min(40, n + 1)))} aria-label={t("table.more")} disabled={n >= 40}>
                        <Plus size={15} aria-hidden />
                      </button>
                    </div>
                    <button type="button" className="tablelist__del" onClick={() => setItems(toggleTable(x.id))} aria-label={t("table.remove")}>
                      <X size={16} aria-hidden />
                    </button>
                  </li>
                );
              })}
            </ul>

            <div className="occ__row">
              <span className="occ__label">{t("occ.guests")}</span>
              <div className="counter occ__counter">
                <button type="button" className="counter__btn" onClick={() => setGuests(Math.max(1, guests - 1))} aria-label="−">
                  <Minus size={16} aria-hidden />
                </button>
                <output className="num">{guests}</output>
                <button type="button" className="counter__btn" onClick={() => setGuests(Math.min(40, guests + 1))} aria-label="+">
                  <Plus size={16} aria-hidden />
                </button>
              </div>
            </div>
            <div className="occ__row">
              <span className="occ__label">{t("quiz.country")}</span>
              <Select value={country} onChange={setCountry} searchable options={(meta?.countries ?? []).map((c) => ({ value: c.code, label: `${c.label} · ${c.symbol}` }))} />
            </div>
            <div className="occ__row">
              <span className="occ__label">{t("quiz.store")}</span>
              <Select value={store} onChange={setStore} options={stores.map((s) => ({ value: s.code, label: s.name }))} />
              {store && <span className="occ__mark"><StoreMark code={store} name={stores.find((s) => s.code === store)?.name ?? store} /></span>}
            </div>

            <p className="quiz__hint">{t("table.hint")}</p>
            {error && <p className="error-inline">{error}</p>}
            <button type="button" className="btn btn-primary btn-lg occ__go" onClick={build} disabled={busy || !store}>
              {t("table.build", { n: items.length, dishes: tn("dishes", items.length) })}
              {rough > 0 && <span className="occ__go-sum num"> · {approx(rough, cy, lang)}</span>}
            </button>
            <button type="button" className="btn btn-link btn-sm tablelist__clear" onClick={() => { clearTable(); setItems([]); }}>
              <Trash2 size={14} aria-hidden /> {t("table.clear")}
            </button>
          </>
        )}
      </main>
      <SiteFooter />
    </div>
  );
}
