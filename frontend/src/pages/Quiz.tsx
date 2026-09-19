import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { AlertCircle, ArrowLeft, ArrowRight, Baby, CalendarDays, Check, ChefHat, FolderOpen, Home, MapPin, Plus, RefreshCw, Search, Trash2, Unlock, User, X } from "lucide-react";
import { Link } from "react-router-dom";
import { OccasionIcon } from "./Occasion";
import { SiteFooter } from "../components/SiteFooter";
import { TopBar } from "../components/TopBar";
import { Select } from "../components/Select";
import { StoreMark, WITH_LOGO } from "../components/StoreMark";
import { EquipmentIcon } from "../components/EquipmentIcon";
import { ReceiptLoader } from "../components/ReceiptLoader";
import { Welcome } from "../components/Welcome";
import { api } from "../lib/api";
import { track } from "../lib/analytics";
import { writeJSON } from "../lib/storage";
import { readDraft } from "../lib/draft";
import { weekRange } from "../lib/format";
import type { Child, Country, Member, Meta, OccasionView, Params, PlanSummary } from "../lib/types";
import { APPETITES } from "../lib/types";
import { useAuth } from "../lib/auth";
import { intlLocale, langCountry, useT, type Lang } from "../i18n";
import { ageOptions, feedingOptions, formulaMlByAge } from "../lib/kids";

const KEY = "racion.quiz.v4";
const COUNTRY_KEY = "racion.country.chosen"; // страна выбрана вручную, по IP не переопределяем
const REGION_KEY = "racion.region.chosen"; // регион выбрали или убрали сами: по IP больше не подставляем
const PREFILL_KEY = "racion.quiz.prefilled"; // id аккаунта, чьи ответы уже подставлены в черновик
const STEPS = 7;

const DEFAULTS: Params = {
  country: "",
  store: "",
  region: "",
  adults: 2,
  members: [
    { name: "", goal: "", appetite: "normal", slots: [] },
    { name: "", goal: "", appetite: "normal", slots: [] },
  ],
  kids: [],
  goal: "",
  kcalTarget: 0,
  budgetMode: "perPersonDay",
  budgetValue: 0,
  allergens: [],
  exclude: [],
  excludeTags: [],
  equipment: ["stove", "oven", "microwave"],
  slots: ["breakfast", "lunch", "dinner"],
  wants: [],
  have: [],
};

const EMPTY_MEMBER: Member = { name: "", goal: "", appetite: "normal", slots: [] };

function toggle(list: string[], id: string): string[] {
  return list.includes(id) ? list.filter((x) => x !== id) : [...list, id];
}

// Ближайшие понедельники: с какой недели планируем.
function nextMondays(n: number, lang: Lang, thisWeek: string, nextWeek: string): { iso: string; label: string }[] {
  const out: { iso: string; label: string }[] = [];
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  const wd = d.getDay();
  const delta = wd === 1 ? 0 : (8 - wd) % 7;
  d.setDate(d.getDate() + delta);
  const f = (x: Date) => x.toLocaleDateString(intlLocale(lang), { day: "numeric", month: "short" }).replace(/\.$/, "").replace(".", "");
  for (let i = 0; i < n; i++) {
    const a = new Date(d);
    a.setDate(d.getDate() + i * 7);
    const b = new Date(a);
    b.setDate(a.getDate() + 6);
    const iso = `${a.getFullYear()}-${String(a.getMonth() + 1).padStart(2, "0")}-${String(a.getDate()).padStart(2, "0")}`;
    const tail = i === 0 ? ` ${delta === 0 ? thisWeek : nextWeek}` : "";
    out.push({ iso, label: `${f(a)} – ${f(b)}${tail}` });
  }
  return out;
}

// Сумма в валюте страны для подсказок и поля бюджета.
function fmtMoney(v: number, c: Country | undefined): string {
  if (!c) return String(v);
  try {
    return new Intl.NumberFormat(c.locale, { style: "currency", currency: c.currency, minimumFractionDigits: 0, maximumFractionDigits: c.decimals }).format(v);
  } catch {
    return `${v} ${c.symbol}`;
  }
}

export function Quiz() {
  const nav = useNavigate();
  const { user } = useAuth();
  const { t, lang } = useT();
  const [sp, setSp] = useSearchParams();
  const step = Math.min(STEPS, Math.max(1, Number(sp.get("s") ?? 1) || 1));

  const [meta, setMeta] = useState<Meta | null>(null);
  const [metaError, setMetaError] = useState<string | null>(null);
  const [p, setP] = useState<Params>(() => {
    const stored = readDraft();
    const init = { ...DEFAULTS, country: langCountry(lang), ...stored };
    if (!stored.members || stored.members.length === 0) {
      // ответы старого квиза или из аккаунта: N одинаковых взрослых с общей целью
      init.members = Array.from({ length: Math.max(1, init.adults) }, () => ({ ...EMPTY_MEMBER, goal: init.goal }));
    }
    if (!init.wants) init.wants = [];
    if (!init.have) init.have = [];
    return init;
  });
  const AGE_OPTIONS = useMemo(() => ageOptions(t, lang), [t, lang]);
  const limitsCount = p.allergens.length + p.excludeTags.length + p.exclude.length;
  const [busy, setBusy] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [regionQuery, setRegionQuery] = useState("");
  const titleRef = useRef<HTMLHeadingElement>(null);

  // Справочники зависят от языка и страны (пресеты бюджета в валюте).
  useEffect(() => {
    api.meta(p.country).then(setMeta).catch((e: Error) => setMetaError(e.message));
  }, [lang, p.country]);

  // Состав семьи из кабинета подставляется при входе, если он задан; на неделю его можно поправить.
  const [fromFamily, setFromFamily] = useState(false);
  // события (сезонные первыми) и пресет недели по ?event=lent; неделя из коллекции по ?collection=id
  const [occasions, setOccasions] = useState<OccasionView[]>([]);
  useEffect(() => {
    api.occasions(lang).then(setOccasions).catch(() => setOccasions([]));
  }, [lang]);
  const eventId = sp.get("event");
  const eventPreset = eventId ? occasions.find((o) => o.id === eventId && o.kind === "week") : undefined;
  // Пресет действует только на эту неделю: в черновик (localStorage) уходят ответы до пресета,
  // иначе «Пост» оставался бы в ограничениях и портил следующие недели и праздничные столы.
  const beforePreset = useRef<Pick<Params, "excludeTags" | "allergens" | "goal" | "members"> | null>(null);
  useEffect(() => {
    if (!eventPreset?.preset) return;
    setP((prev) => {
      beforePreset.current ??= { excludeTags: prev.excludeTags, allergens: prev.allergens, goal: prev.goal, members: prev.members };
      return { ...prev, excludeTags: eventPreset.preset!.excludeTags, allergens: eventPreset.preset!.allergens, goal: eventPreset.preset!.goal || prev.goal, members: prev.members.map((m) => ({ ...m, goal: eventPreset.preset!.goal || m.goal })) };
    });
    track("occasion_preset", { id: eventPreset.id });
  }, [eventPreset?.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const [collName, setCollName] = useState("");
  // Вошедшему с неделями — быстрые действия вместо анкеты с нуля: открыть последнюю, собрать как в прошлый раз
  const [lastPlan, setLastPlan] = useState<PlanSummary | null>(null);
  useEffect(() => {
    if (!user) {
      setLastPlan(null);
      return;
    }
    api.myPlans().then((list) => setLastPlan(list[0] ?? null)).catch(() => setLastPlan(null));
  }, [user?.id]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const cid = sp.get("collection");
    if (cid) setP((prev) => ({ ...prev, collection: cid }));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!p.collection) return;
    const find = (cs: { id: string; name: string }[]) => cs.find((c) => c.id === p.collection)?.name;
    (user ? api.collections().then(find).catch(() => undefined) : Promise.resolve(undefined)).then((n) => {
      if (n) setCollName(n);
      else api.publicCollections(lang).then((cs) => setCollName(find(cs) ?? "")).catch(() => setCollName(""));
    });
  }, [p.collection, user?.id]);
  useEffect(() => {
    if (!user) return;
    api.family().then((f) => {
      if (f.adults.length === 0 && f.kids.length === 0) return;
      setP((prev) => ({ ...prev, members: f.adults.length ? f.adults.map((m) => ({ ...m, slots: m.slots ?? [] })) : prev.members, adults: f.adults.length || prev.adults, goal: f.adults[0]?.goal ?? prev.goal, kids: f.kids }));
      setFromFamily(true);
    }).catch(() => undefined);
  }, [user?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  // Страна по IP подставляется один раз, пока человек не выбрал страну сам (флаг в localStorage).
  const geoApplied = useRef(false);
  useEffect(() => {
    if (!meta || geoApplied.current) return;
    geoApplied.current = true;
    let chosen = false;
    try {
      chosen = localStorage.getItem(COUNTRY_KEY) === "1";
    } catch {
      // приватный режим
    }
    if (!chosen && meta.geoCountry && meta.geoCountry !== p.country) {
      setP((prev) => ({ ...prev, country: meta.geoCountry, store: "", region: "", budgetValue: 0 }));
      track("quiz_geo_country", { country: meta.geoCountry });
    }
  }, [meta]); // eslint-disable-line react-hooks/exhaustive-deps

  // Регион или город по IP: подставляем, пока поле пустое и человек его не трогал
  useEffect(() => {
    if (!meta?.geoRegion || p.region || meta.geoCountry !== p.country) return;
    let chosen = false;
    try {
      chosen = localStorage.getItem(REGION_KEY) === "1";
    } catch {
      // приватный режим
    }
    if (chosen || !meta.regions.some((r) => r.code === meta.geoRegion)) return;
    setP((prev) => ({ ...prev, region: meta.geoRegion! }));
    track("quiz_geo_region", { region: meta.geoRegion });
  }, [meta]); // eslint-disable-line react-hooks/exhaustive-deps
  const regionChosen = () => {
    try {
      localStorage.setItem(REGION_KEY, "1");
    } catch {
      // приватный режим
    }
  };

  // Страна для SSR-страниц рецептов (цены в валюте): та же cookie читает бэкенд.
  useEffect(() => {
    document.cookie = `racion_country=${p.country};path=/;max-age=31536000;samesite=lax`;
  }, [p.country]);

  useEffect(() => {
    const b = beforePreset.current;
    writeJSON(KEY, b && eventPreset ? { ...p, excludeTags: b.excludeTags, allergens: b.allergens, goal: b.goal, members: p.members.map((m, i) => ({ ...m, goal: b.members[i]?.goal ?? m.goal })) } : p);
  }, [p, eventPreset]);

  // Вошедшему сразу подставляем всё известное: прошлые ответы из аккаунта (магазин, бюджет, техника…),
  // поверх них — состав семьи из кабинета. Локальный черновик остаётся у гостя.
  // Подставляем один раз на аккаунт (флаг racion.quiz.prefilled): иначе правки в черновике — например,
  // другая страна — откатывались бы при каждом обновлении страницы.
  useEffect(() => {
    if (!user || !user.defaults || Object.keys(user.defaults).length === 0) return;
    let done = "";
    try {
      done = localStorage.getItem(PREFILL_KEY) ?? "";
    } catch {
      // без хранилища подставляем каждый раз
    }
    if (done === user.id) return;
    const d = user.defaults as Partial<Params>;
    setP((prev) => ({ ...prev, ...d, members: d.members && d.members.length ? d.members : prev.members, wants: d.wants ?? [], have: d.have ?? prev.have ?? [], kids: d.kids ?? prev.kids }));
    try {
      localStorage.setItem(PREFILL_KEY, user.id);
    } catch {
      // приватный режим
    }
  }, [user?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    track("quiz_step", { step });
    titleRef.current?.focus({ preventScroll: true });
    window.scrollTo({ top: 0 });
  }, [step]);

  const go = useCallback((n: number) => setSp({ s: String(n) }), [setSp]);
  const set = (patch: Partial<Params>) => setP((prev) => ({ ...prev, ...patch }));

  const country = useMemo(() => meta?.countries.find((c) => c.code === p.country), [meta, p.country]);
  const stores = useMemo(() => meta?.stores.filter((s) => s.country === p.country) ?? [], [meta, p.country]);
  // Семья: список едоков; adults и goal хранятся для совместимости и выводятся из списка.
  const setMembers = (members: Member[]) => set({ members, adults: members.length, goal: members[0]?.goal ?? p.goal });
  const updateMember = (i: number, patch: Partial<Member>) => setMembers(p.members.map((m, j) => (j === i ? { ...m, ...patch } : m)));
  const setMemberGoal = (i: number, goal: string) => {
    const members = p.members.map((m, j) => (j === i ? { ...m, goal } : m));
    set({ members, adults: members.length, goal: members[0].goal, kcalTarget: members.length > 1 ? 0 : p.kcalTarget });
  };
  const toggleMemberSlot = (i: number, slot: string) => {
    const m = p.members[i];
    const all = meta?.slots.map((s) => s.id) ?? [];
    const cur = m.slots.length === 0 ? all : m.slots;
    const next = cur.includes(slot) ? cur.filter((x) => x !== slot) : [...cur, slot];
    if (next.length === 0) return; // хотя бы один приём дома
    updateMember(i, { slots: next.length === all.length ? [] : next });
  };
  const [wantQuery, setWantQuery] = useState("");
  const wantSuggestions = useMemo(() => {
    const q = wantQuery.trim().toLowerCase();
    if (q.length < 2 || !meta) return [];
    return meta.ingredients.filter((i) => i.label.toLowerCase().includes(q) && !p.wants.includes(i.id) && !p.exclude.includes(i.id)).slice(0, 6);
  }, [wantQuery, meta, p.wants, p.exclude]);
  const [haveQuery, setHaveQuery] = useState("");
  const haveSuggestions = useMemo(() => {
    const q = haveQuery.trim().toLowerCase();
    if (q.length < 2 || !meta) return [];
    return meta.ingredients.filter((i) => i.label.toLowerCase().includes(q) && !p.have.includes(i.id) && !p.exclude.includes(i.id)).slice(0, 6);
  }, [haveQuery, meta, p.have, p.exclude]);

  const setCountry = (code: string) => {
    if (code === p.country) return;
    try {
      localStorage.setItem(COUNTRY_KEY, "1");
    } catch {
      // приватный режим
    }
    set({ country: code, store: "", region: "", budgetValue: 0 });
    track("quiz_country", { country: code });
  };

  const canNext = useMemo(() => {
    switch (step) {
      case 1:
        return p.store !== "";
      case 3:
        return p.members.every((m) => m.goal !== "");
      case 6:
        return p.equipment.length > 0 && p.slots.length > 0;
      case 7:
        return p.budgetValue > 0;
      default:
        return true;
    }
  }, [step, p]);

  const submit = async () => {
    if (busy) return;
    setBusy(true);
    setSubmitError(null);
    track("plan_submit", { country: p.country, store: p.store, region: p.region, adults: p.adults, kids: p.kids.length, goal: p.goal, budgetMode: p.budgetMode, budgetValue: p.budgetValue });
    try {
      const plan = await api.createPlan(p);
      track("plan_created", { id: plan.id, cost: plan.totals.cost });
      if (user) api.updateMe({ defaults: p }).catch(() => {});
      nav(`/plan/${plan.id}`, { state: { plan } });
    } catch (e) {
      setSubmitError((e as Error).message);
      setBusy(false);
    }
  };

  const suggestions = useMemo(() => {
    if (!meta || query.trim().length < 2) return [];
    const q = query.trim().toLowerCase();
    const preset = new Set(meta.excludePresets.map((x) => x.id));
    return meta.ingredients.filter((i) => i.label.toLowerCase().includes(q) && !p.exclude.includes(i.id) && !preset.has(i.id)).slice(0, 6);
  }, [meta, query, p.exclude]);

  const regionMatches = useMemo(() => {
    if (!meta) return [];
    const q = regionQuery.trim().toLowerCase();
    if (q.length < 2) return [];
    const hits = meta.regions.filter((r) => r.name.toLowerCase().includes(q));
    // сначала совпадения с начала названия, потом остальные; регионы раньше городов
    hits.sort((a, b) => {
      const sa = a.name.toLowerCase().startsWith(q) ? 0 : 1;
      const sb = b.name.toLowerCase().startsWith(q) ? 0 : 1;
      if (sa !== sb) return sa - sb;
      if (a.kind !== b.kind) return a.kind === "region" ? -1 : 1;
      return a.name.localeCompare(b.name, "ru");
    });
    return hits.slice(0, 7);
  }, [meta, regionQuery]);

  const regionName = (code: string) => meta?.regions.find((r) => r.code === code)?.name ?? "";
  const ingName = (id: string) => meta?.ingredients.find((i) => i.id === id)?.label ?? id;
  const presetIds = new Set(meta?.excludePresets.map((x) => x.id) ?? []);
  const extraExcludes = p.exclude.filter((id) => !presetIds.has(id));
  const goalKcal = meta?.goals.find((g) => g.id === p.goal)?.kcal ?? 0;

  const updateKid = (i: number, patch: Partial<Child>) => set({ kids: p.kids.map((k, j) => (j === i ? { ...k, ...patch } : k)) });
  const addKid = () => set({ kids: [...p.kids, { ageMonths: 60, feeding: "shared", sharesMeals: false, formula: false, formulaBrand: "", formulaMl: 0 }] });

  return (
    <div className="shell">
      <TopBar />
      <Welcome />
      <main className="quiz">
        <div className="quiz__progress" role="progressbar" aria-valuemin={1} aria-valuemax={STEPS} aria-valuenow={step} aria-label={t("quiz.step", { n: step, total: STEPS })}>
          {Array.from({ length: STEPS }, (_, i) => (
            <span key={i} className={i + 1 < step ? "is-done" : i + 1 === step ? "is-current" : ""} />
          ))}
        </div>

        {metaError && (
          <p className="error-inline" role="alert">
            <AlertCircle size={18} aria-hidden /> {metaError}
          </p>
        )}

        {step === 1 && (
          <section className="quiz__step" key="s1">
            <h1 className="quiz__title" ref={titleRef} tabIndex={-1}>
              {t("quiz.q1")}
            </h1>
            <p className="quiz__hint">{p.country === "RU" ? t("quiz.q1.hint.RU") : t("quiz.q1.hint")}</p>
            {user && lastPlan && !eventPreset && !p.collection && (
              <div className="quickstart" aria-label={t("quick.title")}>
                <Link className="quickstart__card" to={`/plan/${lastPlan.id}`} onClick={() => track("quick_open_plan")}>
                  <span className="quickstart__icon"><CalendarDays size={20} aria-hidden /></span>
                  <span><b>{t("quick.open")}</b><small>{lastPlan.title || t("plan.title", { range: weekRange(lastPlan.startDate, lang) })} · {lastPlan.items} {t("quick.items")}{lastPlan.checked > 0 ? ` · ${t("quick.bought", { n: lastPlan.checked })}` : ""}</small></span>
                </Link>
                <button type="button" className="quickstart__card" disabled={busy || !p.store} onClick={() => { track("quick_rebuild"); void submit(); }}>
                  <span className="quickstart__icon"><RefreshCw size={20} aria-hidden /></span>
                  <span><b>{t("quick.rebuild")}</b><small>{t("quick.rebuild.sub", { store: stores.find((s) => s.code === p.store)?.name ?? p.store })}</small></span>
                </button>
              </div>
            )}
            {eventPreset && (
              <p className="quiz__preset">
                <OccasionIcon name={eventPreset.icon} size={15} /> {eventPreset.title}: {eventPreset.lead}
              </p>
            )}
            {p.collection && collName && (
              <p className="quiz__preset">
                <FolderOpen size={15} aria-hidden /> {t("quiz.collection", { name: collName })} — {t("quiz.collection.hint")}
                <button type="button" className="btn btn-link btn-sm" onClick={() => set({ collection: undefined })}>{t("cancel")}</button>
              </p>
            )}
            {occasions.length > 0 && !eventPreset && (
              <div className="quiz__occ">
                <span className="quiz__occ-label">{t("occ.pick")}</span>
                <div className="chips">
                  {occasions.filter((o) => !o.countries?.length || o.countries.includes(p.country)).map((o) => (
                    <Link key={o.id} to={`/event/${o.id}`} className={"chip chip--sm" + (o.season ? " chip--season" : "")} onClick={() => track("occasion_open", { id: o.id })}>
                      <OccasionIcon name={o.icon} size={14} /> {o.title}
                    </Link>
                  ))}
                </div>
              </div>
            )}
            <div className="quiz__group quiz__group--tight">
              <label className="quiz__label" htmlFor="country">
                {t("quiz.country")}
              </label>
              <Select id="country" value={p.country} onChange={setCountry} searchable options={(meta?.countries ?? []).map((c) => ({ value: c.code, label: `${c.label} · ${c.symbol}` }))} />
            </div>
            <div className="tiles" role="radiogroup" aria-label={t("quiz.store")}>
              {meta
                ? stores.map((s) => (
                    <button key={s.code} type="button" role="radio" aria-checked={p.store === s.code} className={"tile" + (p.store === s.code ? " is-on" : "")} onClick={() => set({ store: s.code })}>
                      <span className={"tile__logo" + (WITH_LOGO.has(s.code) ? " tile__logo--plate" : "")}>
                        <StoreMark code={s.code} name={s.name} />
                      </span>
                      <span>
                        <span className="visually-hidden">{s.name}</span>
                        <span className="tile__sub">{s.note}</span>
                      </span>
                    </button>
                  ))
                : Array.from({ length: 9 }, (_, i) => <div key={i} className="skeleton" style={{ minHeight: 88 }} />)}
            </div>
            {country?.hasRegions && (
            <div className="quiz__group">
              <label className="quiz__label" htmlFor="region-search">
                {t("quiz.region")}
              </label>
              {p.region ? (
                <div className="chips">
                  <button type="button" className="chip chip--remove" onClick={() => { regionChosen(); set({ region: "" }); }} aria-label={t("quiz.region.reset", { name: regionName(p.region) })}>
                    <MapPin size={14} aria-hidden /> {regionName(p.region) || t("quiz.region.all")}
                    <X size={14} aria-hidden />
                  </button>
                </div>
              ) : (
                <>
                  <div className="search">
                    <Search size={18} aria-hidden />
                    <input id="region-search" className="form-control" placeholder={t("quiz.region.placeholder")} value={regionQuery} onChange={(e) => setRegionQuery(e.target.value)} autoComplete="off" />
                  </div>
                  {regionMatches.length > 0 && (
                    <div className="suggest" role="listbox" aria-label={t("quiz.region.list")}>
                      {regionMatches.map((r) => (
                        <button
                          key={r.code}
                          type="button"
                          role="option"
                          aria-selected={false}
                          onClick={() => {
                            regionChosen();
                            set({ region: r.code });
                            setRegionQuery("");
                          }}
                        >
                          {r.name}
                          {r.kind === "city" && <span className="suggest__sub"> · {regionName(r.parent)}</span>}
                        </button>
                      ))}
                    </div>
                  )}
                  <p className="quiz__hint">{t("quiz.region.hint")}</p>
                </>
              )}
            </div>
            )}
          </section>
        )}

        {step === 2 && (
          <section className="quiz__step" key="s2">
            <h1 className="quiz__title" ref={titleRef} tabIndex={-1}>
              {t("quiz.q2")}
            </h1>
            <p className="quiz__hint">{fromFamily ? t("quiz.family.from") : t("quiz.q2.hint")}</p>
            <div className="quiz__group">
              {p.members.map((m, i) => (
                <div className="kid member" key={i}>
                  <div className="kid__head">
                    <label className="member__name">
                      <User size={18} aria-hidden />
                      <input className="form-control" value={m.name} maxLength={40} placeholder={t("quiz.member.name", { n: i + 1 })} onChange={(e) => updateMember(i, { name: e.target.value })} aria-label={t("quiz.member.name.aria", { n: i + 1 })} />
                    </label>
                    {p.members.length > 1 && (
                      <button type="button" className="kid__remove" onClick={() => setMembers(p.members.filter((_, j) => j !== i))} aria-label={t("quiz.member.remove")}>
                        <Trash2 size={16} aria-hidden />
                      </button>
                    )}
                  </div>
                  <div className="member__row">
                    <span className="member__label">{t("quiz.appetite")}</span>
                    <div className="segmented segmented--3 member__seg" role="radiogroup" aria-label={t("quiz.appetite")}>
                      {APPETITES.map((a) => (
                        <button key={a} type="button" role="radio" aria-checked={m.appetite === a} onClick={() => updateMember(i, { appetite: a })}>
                          {t(`appetite.${a}`)}
                        </button>
                      ))}
                    </div>
                  </div>
                  <div className="member__row">
                    <span className="member__label">{t("quiz.member.slots")}</span>
                    <div className="chips" role="group" aria-label={t("quiz.member.slots")}>
                      {meta?.slots.map((s) => {
                        const on = m.slots.length === 0 || m.slots.includes(s.id);
                        return (
                          <button key={s.id} type="button" className="chip chip--sm" aria-pressed={on} onClick={() => toggleMemberSlot(i, s.id)}>
                            {s.label}
                          </button>
                        );
                      })}
                    </div>
                  </div>
                </div>
              ))}
              {p.members.length < 8 && (
                <button type="button" className="btn btn-soft" onClick={() => setMembers([...p.members, { ...EMPTY_MEMBER }])}>
                  <Plus size={18} aria-hidden /> {t("quiz.member.add")}
                </button>
              )}
              {p.kids.map((k, i) => (
                <div className="kid" key={i}>
                  <div className="kid__head">
                    <span className="kid__title">
                      <Baby size={18} aria-hidden /> {t("quiz.child")} {p.kids.length > 1 ? i + 1 : ""}
                    </span>
                    <button type="button" className="kid__remove" onClick={() => set({ kids: p.kids.filter((_, j) => j !== i) })} aria-label={t("quiz.child.remove")}>
                      <Trash2 size={16} aria-hidden />
                    </button>
                  </div>
                  <label className="kid__row">
                    <span>{t("quiz.age")}</span>
                    <Select
                      aria-label={t("quiz.age")}
                      value={String(k.ageMonths)}
                      options={AGE_OPTIONS.map((a) => ({ value: String(a.months), label: a.label }))}
                      onChange={(v) => {
                        const m = Number(v);
                        const opts = feedingOptions(m);
                        updateKid(i, { ageMonths: m, feeding: opts.includes(k.feeding) ? k.feeding : opts[0], sharesMeals: false, formula: m < 36 ? k.formula : false });
                      }}
                    />
                  </label>
                  {k.ageMonths >= 6 && (
                    <div className="kid__feeding" role="radiogroup" aria-label={t("quiz.feeding")}>
                      {feedingOptions(k.ageMonths).map((f) => (
                        <button key={f} type="button" role="radio" aria-checked={k.feeding === f} className="option option--compact" onClick={() => updateKid(i, { feeding: f })}>
                          <span>
                            <span className="option__title">{t(`feeding.${f}`)}</span>
                            <span className="option__sub">{t(`feeding.${f}.sub`)}</span>
                          </span>
                        </button>
                      ))}
                    </div>
                  )}
                  {k.ageMonths < 36 && (
                    <Switch label={t("quiz.formula")} sub={k.ageMonths < 6 ? t("quiz.formula.sub.young") : t("quiz.formula.sub")} checked={k.formula} onChange={(v) => updateKid(i, { formula: v, formulaBrand: v && !k.formulaBrand ? "nutrilon" : k.formulaBrand })} />
                  )}
                  {k.formula && k.ageMonths < 36 && (
                    <>
                      <label className="kid__row">
                        <span>{t("quiz.formula.brand")}</span>
                        <Select aria-label={t("quiz.formula.brand")} value={k.formulaBrand} onChange={(v) => updateKid(i, { formulaBrand: v })} options={(meta?.formulaBrands ?? []).map((b) => ({ value: b.id, label: b.name, sub: b.note }))} />
                      </label>
                      <label className="kid__row">
                        <span>
                          {t("quiz.formula.ml")} <small>{t("quiz.formula.ml.sub", { n: formulaMlByAge(k.ageMonths) })}</small>
                        </span>
                        <input className="form-control kid__num" type="number" inputMode="numeric" min={100} max={1500} step={50} placeholder={String(formulaMlByAge(k.ageMonths))} value={k.formulaMl || ""} onChange={(e) => updateKid(i, { formulaMl: Number(e.target.value) || 0 })} />
                      </label>
                    </>
                  )}
                </div>
              ))}
              {p.kids.length < 8 && (
                <button type="button" className="btn btn-soft" onClick={addKid}>
                  <Plus size={18} aria-hidden /> {t("quiz.child.add")}
                </button>
              )}
            </div>
          </section>
        )}

        {step === 3 && (
          <section className="quiz__step" key="s3">
            <h1 className="quiz__title" ref={titleRef} tabIndex={-1}>
              {t("quiz.q3")}
            </h1>
            <p className="quiz__hint">{t("quiz.q3.hint")}</p>
            {p.members.map((m, i) => (
              <div className="quiz__group member__goals" key={i}>
                {p.members.length > 1 && <div className="quiz__label">{m.name || t("quiz.member.name", { n: i + 1 })}</div>}
                <div className="options" role="radiogroup" aria-label={m.name || t("quiz.member.name", { n: i + 1 })}>
                  {meta?.goals.map((g) => (
                    <button key={g.id} type="button" role="radio" aria-checked={m.goal === g.id} className={"option" + (p.members.length > 1 ? " option--compact" : "")} onClick={() => setMemberGoal(i, g.id)}>
                      <span>
                        <span className="option__title">{g.label}</span>
                        {p.members.length === 1 && <span className="option__sub">{t(`goal.${g.id}.sub`)}</span>}
                      </span>
                      {g.kcal > 0 && <span className="option__val num">{g.kcal} {t("kcal")}</span>}
                    </button>
                  ))}
                </div>
              </div>
            ))}
            {p.members.length === 1 && p.goal && p.goal !== "none" && (
              <div className="quiz__group">
                <label className="quiz__label" htmlFor="kcal">
                  {t("quiz.kcal")} <small>{t("quiz.optional")}</small>
                </label>
                <input id="kcal" className="form-control" type="number" inputMode="numeric" min={1000} max={6000} step={50} placeholder={String(goalKcal)} value={p.kcalTarget || ""} onChange={(e) => set({ kcalTarget: Number(e.target.value) || 0 })} />
              </div>
            )}
          </section>
        )}

        {step === 4 && (
          <section className="quiz__step" key="s4">
            <h1 className="quiz__title" ref={titleRef} tabIndex={-1}>
              {t("quiz.q4")}
            </h1>
            <p className="quiz__hint">{t("quiz.q4.hint")}</p>
            {limitsCount > 0 && (
              <button type="button" className="btn btn-soft btn-sm quiz__clear" onClick={() => set({ allergens: [], excludeTags: [], exclude: [] })}>
                <Unlock size={15} aria-hidden /> {t("quiz.limits.clear", { n: limitsCount })}
              </button>
            )}
            <div className="chips" role="group" aria-label={t("quiz.allergens")}>
              <button type="button" className="chip" aria-pressed={p.allergens.length === 0} onClick={() => set({ allergens: [] })}>
                {p.allergens.length === 0 && <Check size={16} aria-hidden />}
                {t("quiz.allergens.none")}
              </button>
              {meta?.allergens.map((a) => (
                <button key={a.id} type="button" className="chip" aria-pressed={p.allergens.includes(a.id)} onClick={() => set({ allergens: toggle(p.allergens, a.id) })}>
                  {p.allergens.includes(a.id) && <Check size={16} aria-hidden />}
                  {a.label}
                </button>
              ))}
            </div>
          </section>
        )}

        {step === 5 && (
          <section className="quiz__step" key="s5">
            <h1 className="quiz__title" ref={titleRef} tabIndex={-1}>
              {t("quiz.q5")}
            </h1>
            <p className="quiz__hint">{t("quiz.q5.hint")}</p>
            {limitsCount > 0 && (
              <button type="button" className="btn btn-soft btn-sm quiz__clear" onClick={() => set({ allergens: [], excludeTags: [], exclude: [] })}>
                <Unlock size={15} aria-hidden /> {t("quiz.limits.clear", { n: limitsCount })}
              </button>
            )}
            <div className="chips" role="group" aria-label={t("quiz.exclude.presets")}>
              {meta?.excludePresets.map((x) => {
                const on = x.kind === "tag" ? p.excludeTags.includes(x.id) : p.exclude.includes(x.id);
                return (
                  <button key={x.id} type="button" className="chip" aria-pressed={on} onClick={() => (x.kind === "tag" ? set({ excludeTags: toggle(p.excludeTags, x.id) }) : set({ exclude: toggle(p.exclude, x.id) }))}>
                    {on && <X size={16} aria-hidden />}
                    {x.label}
                  </button>
                );
              })}
            </div>
            <div className="quiz__group">
              <label className="quiz__label" htmlFor="exclude-search">
                {t("quiz.exclude.other")}
              </label>
              <div className="search">
                <Search size={18} aria-hidden />
                <input id="exclude-search" className="form-control" placeholder={t("quiz.exclude.placeholder")} value={query} onChange={(e) => setQuery(e.target.value)} autoComplete="off" />
              </div>
              {suggestions.length > 0 && (
                <div className="suggest" role="listbox" aria-label={t("quiz.exclude.matches")}>
                  {suggestions.map((s) => (
                    <button
                      key={s.id}
                      type="button"
                      role="option"
                      aria-selected={false}
                      onClick={() => {
                        set({ exclude: [...p.exclude, s.id] });
                        setQuery("");
                      }}
                    >
                      {s.label}
                    </button>
                  ))}
                </div>
              )}
              {query.trim().length >= 2 && suggestions.length === 0 && <p className="quiz__hint">{t("quiz.exclude.nomatch")}</p>}
              {extraExcludes.length > 0 && (
                <div className="chips" aria-label={t("quiz.exclude.list")}>
                  {extraExcludes.map((id) => (
                    <button key={id} type="button" className="chip chip--remove" onClick={() => set({ exclude: p.exclude.filter((x) => x !== id) })} aria-label={t("quiz.exclude.remove", { name: ingName(id) })}>
                      {ingName(id)}
                      <X size={14} aria-hidden />
                    </button>
                  ))}
                </div>
              )}
            </div>
            <div className="quiz__group quiz__group--sep">
              <label className="quiz__label" htmlFor="want-search">
                {t("quiz.wants")} <small>{t("quiz.optional")}</small>
              </label>
              <p className="quiz__hint quiz__hint--tight">{t("quiz.wants.hint")}</p>
              <div className="search">
                <Search size={18} aria-hidden />
                <input id="want-search" className="form-control" placeholder={t("quiz.wants.placeholder")} value={wantQuery} onChange={(e) => setWantQuery(e.target.value)} autoComplete="off" />
              </div>
              {wantSuggestions.length > 0 && (
                <div className="suggest" role="listbox" aria-label={t("quiz.wants")}>
                  {wantSuggestions.map((s) => (
                    <button key={s.id} type="button" role="option" aria-selected={false} onClick={() => { set({ wants: [...p.wants, s.id] }); setWantQuery(""); }}>
                      {s.label}
                    </button>
                  ))}
                </div>
              )}
              {p.wants.length > 0 && (
                <div className="chips">
                  {p.wants.map((id) => (
                    <button key={id} type="button" className="chip chip--remove chip--want" onClick={() => set({ wants: p.wants.filter((x) => x !== id) })} aria-label={t("quiz.wants.remove", { name: ingName(id) })}>
                      {ingName(id)} <X size={14} aria-hidden />
                    </button>
                  ))}
                </div>
              )}
            </div>
            <div className="quiz__group quiz__group--sep">
              <label className="quiz__label" htmlFor="have-search">
                {t("quiz.have")} <small>{t("quiz.optional")}</small>
              </label>
              <p className="quiz__hint quiz__hint--tight">{t("quiz.have.hint")}</p>
              <div className="search">
                <Search size={18} aria-hidden />
                <input id="have-search" className="form-control" placeholder={t("quiz.have.placeholder")} value={haveQuery} onChange={(e) => setHaveQuery(e.target.value)} autoComplete="off" />
              </div>
              {haveSuggestions.length > 0 && (
                <div className="suggest" role="listbox" aria-label={t("quiz.have")}>
                  {haveSuggestions.map((s) => (
                    <button key={s.id} type="button" role="option" aria-selected={false} onClick={() => { set({ have: [...p.have, s.id] }); setHaveQuery(""); }}>
                      {s.label}
                    </button>
                  ))}
                </div>
              )}
              {p.have.length > 0 && (
                <div className="chips">
                  {p.have.map((id) => (
                    <button key={id} type="button" className="chip chip--remove chip--have" onClick={() => set({ have: p.have.filter((x) => x !== id) })} aria-label={t("quiz.have.remove", { name: ingName(id) })}>
                      <Home size={13} aria-hidden /> {ingName(id)} <X size={14} aria-hidden />
                    </button>
                  ))}
                </div>
              )}
            </div>
          </section>
        )}

        {step === 6 && (
          <section className="quiz__step" key="s6">
            <h1 className="quiz__title" ref={titleRef} tabIndex={-1}>
              {t("quiz.q6")}
            </h1>
            <p className="quiz__hint">{t("quiz.q6.hint")}</p>
            <div className="tiles tiles--eq" role="group" aria-label={t("quiz.equipment")}>
              {meta?.equipment.map((e) => {
                const on = p.equipment.includes(e.id);
                return (
                  <button key={e.id} type="button" className={"tile tile--eq" + (on ? " is-on" : "")} aria-pressed={on} onClick={() => set({ equipment: toggle(p.equipment, e.id) })}>
                    <span className="tile__icon">
                      <EquipmentIcon id={e.id} on={on} size={26} />
                    </span>
                    <span>{e.label}</span>
                  </button>
                );
              })}
            </div>
            <div className="quiz__group">
              <p className="quiz__label">{t("quiz.slots")}</p>
              <div className="chips" role="group" aria-label={t("quiz.slots.aria")}>
                {meta?.slots.map((s) => (
                  <button key={s.id} type="button" className="chip" aria-pressed={p.slots.includes(s.id)} onClick={() => set({ slots: toggle(p.slots, s.id) })}>
                    {p.slots.includes(s.id) && <Check size={16} aria-hidden />}
                    {s.label}
                  </button>
                ))}
              </div>
              {p.slots.length === 0 && (
                <p className="error-inline">
                  <AlertCircle size={18} aria-hidden /> {t("quiz.slots.min")}
                </p>
              )}
            </div>
          </section>
        )}

        {step === 7 && (
          <section className="quiz__step" key="s7">
            <h1 className="quiz__title" ref={titleRef} tabIndex={-1}>
              {t("quiz.q7")}
            </h1>
            <p className="quiz__hint">{t("quiz.q7.hint")}</p>
            <div className="segmented" role="radiogroup" aria-label={t("quiz.budget.mode")}>
              <button type="button" role="radio" aria-checked={p.budgetMode === "perPersonDay"} onClick={() => set({ budgetMode: "perPersonDay", budgetValue: 0 })}>
                {t("quiz.budget.perDay")}
              </button>
              <button type="button" role="radio" aria-checked={p.budgetMode === "week"} onClick={() => set({ budgetMode: "week", budgetValue: 0 })}>
                {t("quiz.budget.week")}
              </button>
            </div>
            <div className="money">
              <input
                id="budget"
                className="form-control money__input num"
                type="number"
                inputMode="numeric"
                min={0}
                step={country && country.decimals > 0 ? 0.5 : 1}
                placeholder={country ? String(p.budgetMode === "week" ? Math.round((country.default ?? 0) * p.adults * 7) : (country.default ?? 0)) : ""}
                value={p.budgetValue || ""}
                onChange={(e) => set({ budgetValue: Number(e.target.value) || 0 })}
                aria-label={p.budgetMode === "week" ? t("quiz.budget.aria.week") : t("quiz.budget.aria.day")}
              />
              <span className="money__unit">
                {country?.symbol ?? "₽"} {p.budgetMode === "week" ? t("quiz.budget.unit.week") : t("quiz.budget.unit.day")}
              </span>
            </div>
            <div className="chips" aria-label={t("quiz.budget.presets")}>
              {meta?.budgetPresets.map((b) => {
                const eaters = p.adults + p.kids.filter((k) => k.feeding === "shared" || k.feeding === "separate").length * 0.5;
                // недельная подсказка округляется до «круглой» суммы в валюте
                const roundTo = country && country.decimals > 0 ? 5 : b.perDay >= 1000 ? 1000 : 100;
                const value = p.budgetMode === "week" ? Math.round((b.perDay * eaters * 7) / roundTo) * roundTo : b.perDay;
                return (
                  <button key={b.id} type="button" className="chip" aria-pressed={p.budgetValue === value} onClick={() => set({ budgetValue: value })}>
                    {b.label} · <span className="num">{fmtMoney(value, country)}</span>
                  </button>
                );
              })}
            </div>
            <div className="quiz__group">
              <Switch label={t("quiz.compact")} sub={t("quiz.compact.sub")} checked={!!p.compact} onChange={(v) => set({ compact: v })} />
            </div>
            <div className="quiz__group">
              <label className="quiz__label" htmlFor="start">
                {t("quiz.week")}
              </label>
              <Select id="start" value={p.startDate ?? ""} onChange={(v) => set({ startDate: v || undefined })} options={nextMondays(4, lang, t("quiz.week.this"), t("quiz.week.next")).map((m, i) => ({ value: i === 0 ? "" : m.iso, label: m.label }))} />
            </div>
            {submitError && (
              <p className="error-inline" role="alert">
                <AlertCircle size={18} aria-hidden /> {submitError}
              </p>
            )}
          </section>
        )}
      </main>

      <div className="actionbar actionbar--flow">
        <div className="actionbar__inner">
          {step > 1 && (
            <button type="button" className="btn btn-ghost" onClick={() => go(step - 1)} aria-label={t("back")}>
              <ArrowLeft size={20} aria-hidden />
            </button>
          )}
          {step < STEPS ? (
            <button type="button" className="btn btn-primary" disabled={!canNext || !meta} onClick={() => go(step + 1)}>
              {t("next")}
              <ArrowRight size={18} aria-hidden />
            </button>
          ) : (
            <button type="button" className="btn btn-primary" disabled={!canNext || busy} onClick={submit} aria-busy={busy}>
              {busy ? (
                <>
                  <ReceiptLoader label={t("quiz.busy")} /> {t("quiz.busy")}
                </>
              ) : (
                <>
                  <ChefHat size={18} aria-hidden /> {t("quiz.submit")}
                </>
              )}
            </button>
          )}
        </div>
      </div>
      <SiteFooter />
    </div>
  );
}

function Switch({ label, sub, checked, onChange }: { label: string; sub?: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <button type="button" className="switch" role="switch" aria-checked={checked} onClick={() => onChange(!checked)}>
      <span className="switch__text">
        {label}
        {sub && <span className="switch__sub">{sub}</span>}
      </span>
      <span className="switch__track" aria-hidden>
        <span className="switch__knob" />
      </span>
    </button>
  );
}
