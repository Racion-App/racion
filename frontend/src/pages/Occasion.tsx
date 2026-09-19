import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { AlertTriangle, ArrowLeft, Beer, Cake, Circle, Drumstick, Egg, Flower2, Ghost, Gift, Heart, Leaf, Minus, Plus, Sparkles, Sun, Users } from "lucide-react";
import { SiteFooter } from "../components/SiteFooter";
import { TopBar } from "../components/TopBar";
import { Select } from "../components/Select";
import { StoreMark } from "../components/StoreMark";
import { api } from "../lib/api";
import { track } from "../lib/analytics";
import { clearDraftLimits, readDraft } from "../lib/draft";
import type { Meta, OccasionView, Params } from "../lib/types";
import { langCountry, useT, useLang } from "../i18n";

// Событие вместо недели: гости, страна и магазин (из черновика квиза) → «Собрать стол» → чек события.
// Для событий-недель (Пост) кнопка ведёт в квиз с готовыми ограничениями.

const ICONS = { sparkles: Sparkles, users: Users, cake: Cake, sun: Sun, circle: Circle, leaf: Leaf, egg: Egg, ghost: Ghost, drumstick: Drumstick, gift: Gift, heart: Heart, flower: Flower2, beer: Beer } as const;

export function OccasionIcon({ name, size = 18 }: { name: string; size?: number }) {
  const I = ICONS[name as keyof typeof ICONS] ?? Sparkles;
  return <I size={size} aria-hidden />;
}

export function Occasion() {
  const { id = "" } = useParams<{ id: string }>();
  const nav = useNavigate();
  const { t, tn, lang } = useT();
  const { ready } = useLang();
  const [list, setList] = useState<OccasionView[] | null>(null);
  const [draft, setDraft] = useState<Partial<Params>>(() => readDraft());
  const [country, setCountry] = useState(draft.country || langCountry(lang));
  const [store, setStore] = useState(draft.store || "");
  const [guests, setGuests] = useState(0);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.occasions(lang).then(setList).catch(() => setList([]));
  }, [lang]);
  useEffect(() => {
    api.meta(country).then((m) => {
      setMeta(m);
      const own = m.stores.filter((s) => s.country === country);
      if (!own.some((s) => s.code === store)) setStore(own[0]?.code ?? "");
    }).catch(() => setMeta(null));
  }, [country]); // eslint-disable-line react-hooks/exhaustive-deps
  const occ = list?.find((o) => o.id === id);
  useEffect(() => {
    if (occ && guests === 0) setGuests(occ.guests);
  }, [occ, guests]);
  if (!ready) return null;
  if (list && !occ) return <div className="shell"><TopBar /><main className="state errpage"><div className="state__box"><h2>{t("page.404.title")}</h2><Link className="btn btn-primary" to="/">{t("page.err.home")}</Link></div></main></div>;

  const build = async () => {
    if (!occ) return;
    if (occ.kind === "week") {
      track("occasion_week", { id });
      nav(`/?s=1&event=${id}`);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      // анкету не проходили — считаем, что плита, духовка и микроволновка есть (как в квизе по умолчанию)
      const equipment = draft.equipment?.length ? draft.equipment : ["stove", "oven", "microwave"];
      const plan = await api.createOccasion(id, guests, { ...draft, equipment, country, store, region: draft.region ?? "" } as Params);
      track("occasion_built", { id, guests });
      nav(`/plan/${plan.id}`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  // магазины только выбранной страны, как в квизе
  const stores = (meta?.stores ?? []).filter((s) => s.country === country);
  // Ограничения из анкеты действуют и на праздничный стол — показываем, чтобы было ясно, почему нет блинов или мяса
  const limits: string[] = [
    ...(meta?.excludePresets ?? []).filter((x) => (x.kind === "tag" ? draft.excludeTags?.includes(x.id) : draft.exclude?.includes(x.id))).map((x) => x.label),
    ...(draft.excludeTags ?? []).filter((id) => !meta?.excludePresets.some((x) => x.kind === "tag" && x.id === id)).map((id) => t("tag." + id)),
    ...(meta?.allergens ?? []).filter((a) => draft.allergens?.includes(a.id)).map((a) => a.label),
  ];

  return (
    <div className="shell">
      <TopBar />
      <main className="occ">
        <Link to="/" className="occ__back">
          <ArrowLeft size={16} aria-hidden /> {t("home")}
        </Link>
        {!occ && <div className="skeleton" style={{ height: 160 }} />}
        {occ && (
          <>
            <div className="occ__head">
              <span className="occ__icon" aria-hidden>
                <OccasionIcon name={occ.icon} size={26} />
              </span>
              <div>
                <h1 className="occ__title">{occ.title}</h1>
                <p className="occ__lead">{occ.lead}</p>
              </div>
            </div>
            {occ.kind === "menu" && (
              <>
                <div className="occ__courses">
                  {occ.courses.map((c) => (
                    <span key={c} className="chip chip--sm">{c}</span>
                  ))}
                </div>
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
                {limits.length > 0 && (
                  <div className="occ__limits" role="status">
                    <AlertTriangle size={16} aria-hidden />
                    <span>
                      {t("occ.limits", { list: limits.join(", ") })} {t("occ.limits.why")}
                    </span>
                    <span className="occ__limits-actions">
                      <button type="button" className="btn btn-soft btn-sm" onClick={() => { setDraft(clearDraftLimits()); track("occasion_limits_clear", { id }); }}>{t("occ.limits.clear")}</button>
                      <Link className="btn btn-link btn-sm" to="/?s=4">{t("occ.limits.edit")}</Link>
                    </span>
                  </div>
                )}
                <p className="quiz__hint">{t("occ.hint")}</p>
              </>
            )}
            {error && <p className="error-inline">{error}</p>}
            <button type="button" className="btn btn-primary btn-lg occ__go" onClick={build} disabled={busy || (occ.kind === "menu" && !store)}>
              {occ.kind === "week" ? t("occ.build.week") : `${t("occ.build")} ${t("occ.for", { n: guests, guests: tn("guests", guests) })}`}
            </button>
          </>
        )}
      </main>
      <SiteFooter />
    </div>
  );
}
