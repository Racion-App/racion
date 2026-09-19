import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { AlertCircle, ArrowLeft, ArrowLeftRight, Baby, CalendarOff, CalendarPlus, Check, Clock, CopyPlus, Flame, Info, MoreHorizontal, Plus, Printer, RefreshCw, Repeat2, RotateCcw, ScrollText, Share2, ShoppingBasket, ShoppingCart, Sparkles, Store as StoreIcon, Target, ThumbsDown, ThumbsUp, Trash2, Unlock, UserRound, Users, WifiOff } from "lucide-react";
import { SiteFooter } from "../components/SiteFooter";
import { TopBar } from "../components/TopBar";
import { RecipeSheet } from "../components/RecipeSheet";
import { CartSheet } from "../components/CartSheet";
import { flushChecks, pendingCount, queueCheck } from "../lib/offline";
import { api } from "../lib/api";
import { track } from "../lib/analytics";
import { clearDraftLimits } from "../lib/draft";
import { PlanChat } from "../components/PlanChat";
import { IngredientPic } from "../components/IngredientPic";
import { approx, dateShort, minutes, money, people, qty, weekRange } from "../lib/format";
import { readJSON, writeJSON } from "../lib/storage";
import { slotLabel, type Country, type Dish, type Extra, type Params, type Plan as PlanT } from "../lib/types";
import { useAuth } from "../lib/auth";
import { useT } from "../i18n";

export function Plan() {
  const { id = "" } = useParams();
  const loc = useLocation();
  const nav = useNavigate();
  const [sp] = useSearchParams();
  const initial = (loc.state as { plan?: PlanT } | null)?.plan;
  const { t, tn, lang } = useT();

  const [plan, setPlan] = useState<PlanT | null>(initial && initial.id === id ? initial : null);
  const [error, setError] = useState<string | null>(null);
  const [storeMode, setStoreMode] = useState(false);
  const [view, setView] = useState<"all" | "adult" | "kids">("all");
  const { user } = useAuth();
  const [checked, setChecked] = useState<Record<string, boolean>>(() => readJSON(`racion.check.${id}`, {}));
  const [extras, setExtras] = useState<Extra[]>([]);
  const [extraForm, setExtraForm] = useState<{ open: boolean; name: string; qty: string; due: string }>({ open: false, name: "", qty: "", due: "" });
  const [recipeId, setRecipeId] = useState<string | null>(null);
  const [swapping, setSwapping] = useState<string | null>(null);
  const [fresh, setFresh] = useState<string | null>(null);
  // меню «ещё» в нижней панели на телефоне
  const [moreOpen, setMoreOpen] = useState(false);
  const moreRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!moreOpen) return;
    const off = (e: MouseEvent | TouchEvent) => { if (!moreRef.current?.contains(e.target as Node)) setMoreOpen(false); };
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") setMoreOpen(false); };
    document.addEventListener("mousedown", off); document.addEventListener("touchstart", off); document.addEventListener("keydown", esc);
    return () => { document.removeEventListener("mousedown", off); document.removeEventListener("touchstart", off); document.removeEventListener("keydown", esc); };
  }, [moreOpen]);
  const [toast, setToast] = useState<string | null>(null);
  // офлайн: полоска сверху и число недосланных отметок
  const [online, setOnline] = useState(typeof navigator === "undefined" ? true : navigator.onLine);
  const [pending, setPending] = useState(pendingCount());
  useEffect(() => {
    const up = () => { setOnline(true); void flushChecks().then(() => setPending(pendingCount())); };
    const down = () => setOnline(false);
    window.addEventListener("online", up);
    window.addEventListener("offline", down);
    return () => { window.removeEventListener("online", up); window.removeEventListener("offline", down); };
  }, []);
  const printing = useRef(true);
  // Отрыв: тянем линию вверх, список едет за пальцем; отпустили дальше порога — режим магазина.
  const [pull, setPull] = useState<{ dragging: boolean; dy: number; startY: number; id: number | null }>({ dragging: false, dy: 0, startY: 0, id: null });
  const PULL_THRESHOLD = 48;

  // План приходит на языке cookie; при смене языка перечитываем — сервер отдаёт те же числа с новыми подписями.
  // Язык передаём в адресе, а перечитываем только при смене id или языка: раньше эффект зависел от plan,
  // и если сервер отвечал не тем языком (нет cookie), запросы шли по кругу до 429.
  const planRef = useRef(plan);
  planRef.current = plan;
  useEffect(() => {
    const cur = planRef.current;
    if (cur && cur.id === id && cur.lang === lang) return;
    let alive = true;
    api
      .getPlan(id, lang)
      .then((p) => alive && setPlan(p))
      .catch((e: Error) => alive && setError(e.message));
    return () => {
      alive = false;
    };
  }, [id, lang]);

  useEffect(() => {
    if (plan) track("plan_view", { id: plan.id, swaps: plan.swaps });
  }, [plan?.id]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    writeJSON(`racion.check.${id}`, checked);
  }, [checked, id]);

  // Отметки и свои товары хранятся на сервере по ссылке на план; локальная копия — на случай офлайна.
  useEffect(() => {
    api.checks(id).then((ids) => setChecked((prev) => ({ ...prev, ...Object.fromEntries(ids.map((x) => [x, true])) }))).catch(() => {});
    api.extras(id).then(setExtras).catch(() => {});
  }, [id]);

  const toggleCheck = (itemId: string, name: string, qty: string, cost: number) => {
    const next = !checked[itemId];
    setChecked({ ...checked, [itemId]: next });
    api.setCheck(id, { itemId, checked: next, name, qty, cost }).catch(() => {
      // без сети — в очередь, дошлём при появлении связи
      queueCheck({ planId: id, itemId, checked: next, name, qty, cost });
      setPending(pendingCount());
    });
    if (next) track("item_checked", { extra: itemId.startsWith("extra:") });
  };

  const addExtra = async () => {
    const name = extraForm.name.trim();
    if (!name) return;
    try {
      const e = await api.addExtra(id, { name, qty: extraForm.qty.trim(), due: extraForm.due || null });
      setExtras([...extras, e]);
      setExtraForm({ open: false, name: "", qty: "", due: "" });
      track("extra_added");
    } catch (err) {
      setToast((err as Error).message);
    }
  };

  const dislike = async (dish: Dish, day: number) => {
    if (!user) {
      nav(`/login?plan=${id}`);
      return;
    }
    try {
      await api.dislike(dish.recipeId);
      setToast(t("dish.disliked", { title: dish.title }));
      track("dish_disliked", { id: dish.recipeId });
      await swap(day, dish.slot);
    } catch (err) {
      setToast((err as Error).message);
    }
  };

  useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(t);
  }, [toast]);

  // после первой отрисовки «печать» больше не повторяем при свапах
  useEffect(() => {
    const t = window.setTimeout(() => (printing.current = false), 1200);
    return () => window.clearTimeout(t);
  }, []);

  const setMode = useCallback((store: boolean) => {
    const apply = () => {
      setStoreMode(store);
      window.scrollTo({ top: 0 });
    };
    // переход может быть пропущен браузером (уход со страницы, второй переход) — промисы тогда отклоняются, глушим
    const d = document as Document & { startViewTransition?: (cb: () => void) => { finished: Promise<void>; ready: Promise<void>; updateCallbackDone: Promise<void> } };
    if (d.startViewTransition) {
      const tr = d.startViewTransition(apply);
      tr.finished.catch(() => {});
      tr.ready.catch(() => {});
      tr.updateCallbackDone.catch(() => {});
    } else apply();
    track(store ? "store_mode_on" : "store_mode_off");
  }, []);

  const onPullStart = (e: ReactPointerEvent<HTMLDivElement>) => {
    if ((e.target as HTMLElement).closest("button")) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    setPull({ dragging: true, dy: 0, startY: e.clientY, id: e.pointerId });
  };
  const onPullMove = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (!pull.dragging || e.pointerId !== pull.id) return;
    const dy = Math.max(Math.min(0, e.clientY - pull.startY), -140); // только вверх, не дальше 140px
    setPull((p) => ({ ...p, dy }));
  };
  const onPullEnd = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (!pull.dragging || e.pointerId !== pull.id) return;
    const torn = -pull.dy >= PULL_THRESHOLD;
    setPull({ dragging: false, dy: 0, startY: 0, id: null });
    if (torn) {
      track("tear_pull");
      setMode(true);
    }
  };

  // «Переставить»: выбранная ячейка ждёт, куда её поменять; целевые ячейки того же приёма показывают «Сюда»
  const [moving, setMoving] = useState<{ day: number; slot: string } | null>(null);
  const move = async (to: number) => {
    if (!plan || !moving) return;
    const from = moving;
    setMoving(null);
    try {
      const updated = await api.moveDish(plan.id, from.day, to, from.slot);
      setPlan(updated);
      setFresh(`${to}:${from.slot}`);
      window.setTimeout(() => setFresh(null), 700);
      track("dish_move", { from: from.day, to, slot: from.slot });
    } catch (e) {
      setToast((e as Error).message);
    }
  };
  const skipDay = async (day: number, skip: boolean) => {
    if (!plan) return;
    try {
      setPlan(await api.skipDay(plan.id, day, skip));
      track("day_skip", { day, skip });
    } catch (e) {
      setToast((e as Error).message);
    }
  };
  const repeat = async () => {
    if (!plan) return;
    try {
      const copy = await api.repeatPlan(plan.id);
      track("plan_repeat");
      nav(`/plan/${copy.id}`);
      setToast(t("plan.repeat.done"));
    } catch (e) {
      setToast((e as Error).message);
    }
  };

  const swap = async (day: number, slot: string) => {
    if (!plan || swapping) return;
    const key = `${day}:${slot}`;
    setSwapping(key);
    try {
      const updated = await api.swap(plan.id, day, slot);
      setPlan(updated);
      setFresh(key);
      window.setTimeout(() => setFresh(null), 700);
      track("dish_swap", { day, slot });
    } catch (e) {
      setToast((e as Error).message);
    } finally {
      setSwapping(null);
    }
  };

  const swapSide = async (day: number, slot: string) => {
    if (!plan || swapping) return;
    const key = `${day}:${slot}`;
    setSwapping(key);
    try {
      const updated = await api.swapSide(plan.id, day, slot);
      setPlan(updated);
      setFresh(key);
      window.setTimeout(() => setFresh(null), 700);
      track("side_swap", { day, slot });
    } catch (e) {
      setToast((e as Error).message);
    } finally {
      setSwapping(null);
    }
  };

  // Семья: ссылка с ?join=1 — вошедший получает неделю в кабинет и право менять блюда.
  const [joinDone, setJoinDone] = useState(false);
  useEffect(() => {
    if (!plan || !sp.get("join") || joinDone) return;
    if (!user) {
      setToast(t("plan.family.login"));
      return;
    }
    setJoinDone(true);
    api.joinPlan(plan.id).then(() => {
      setToast(t("plan.family.joined"));
      track("plan_join");
      api.getPlan(plan.id).then(setPlan).catch(() => undefined);
    }).catch((e: Error) => setToast(e.message));
  }, [plan?.id, user?.id, sp]); // eslint-disable-line react-hooks/exhaustive-deps

  // Ограничения анкеты (стоп-продукты, теги, аллергены) режут выбор — одной кнопкой снять их и собрать заново
  const hasLimits = !!plan && ((plan.params.excludeTags?.length ?? 0) + (plan.params.allergens?.length ?? 0) + (plan.params.exclude?.length ?? 0) > 0);
  const [rebuilding, setRebuilding] = useState(false);
  const [chatOpen, setChatOpen] = useState(false);
  // помощник по неделе показывается, если на сервере есть нейросети (meta.ai); тянем один раз для вошедшего
  const [aiOn, setAiOn] = useState(false);
  useEffect(() => {
    if (!user) return;
    api.meta().then((m) => setAiOn(!!m.ai)).catch(() => setAiOn(false));
  }, [user?.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const rebuildWithoutLimits = async () => {
    if (!plan) return;
    setRebuilding(true);
    try {
      clearDraftLimits();
      const params = { ...plan.params, excludeTags: [], allergens: [], exclude: [] } as Params;
      const next = plan.occasion ? await api.createOccasion(plan.occasion.id, plan.occasion.guests, params) : await api.createPlan(params);
      track("plan_limits_clear", { occasion: plan.occasion?.id ?? "" });
      nav(`/plan/${next.id}`);
    } catch (e) {
      setToast((e as Error).message);
    } finally {
      setRebuilding(false);
    }
  };

  const invite = async () => {
    const url = `${window.location.origin}/plan/${plan?.id}?join=1`;
    track("plan_invite");
    try {
      if (navigator.share) {
        await navigator.share({ title: t("plan.share.title"), url });
        return;
      }
      await navigator.clipboard.writeText(url);
      setToast(t("plan.family.copied"));
    } catch {
      setToast(t("plan.share.manual"));
    }
  };

  const share = async () => {
    const url = window.location.href;
    track("plan_share");
    try {
      if (navigator.share) {
        await navigator.share({ title: t("plan.share.title"), url });
        return;
      }
      await navigator.clipboard.writeText(url);
      setToast(t("plan.share.copied"));
    } catch {
      setToast(t("plan.share.manual"));
    }
  };

  const bought = useMemo(() => {
    if (!plan) return { n: 0, total: 0 };
    let n = 0;
    let total = 0;
    for (const g of plan.shopping)
      for (const it of g.items) {
        total++;
        if (checked[it.ingredientId]) n++;
      }
    for (const e of extras) {
      total++;
      if (checked[`extra:${e.id}`]) n++;
    }
    return { n, total };
  }, [plan, checked, extras]);

  // «Собрать корзину»: ещё не купленное, без домашнего и «есть дома»
  const [cartOpen, setCartOpen] = useState(false);
  const cartItems = useMemo(() => {
    if (!plan) return [];
    const out: { id: string; name: string; qty: string }[] = [];
    for (const g of plan.shopping)
      for (const it of g.items) {
        if (checked[it.ingredientId] || it.pantry || it.atHome) continue;
        out.push({ id: it.ingredientId, name: it.name, qty: it.loose || it.unit === "pcs" || it.packs <= 1 ? qty(it.buy, it.unit, lang) : `${it.packs} × ${qty(it.pack, it.unit, lang)}` });
      }
    for (const e of extras) if (!checked[`extra:${e.id}`]) out.push({ id: `extra:${e.id}`, name: e.name, qty: e.qty });
    return out;
  }, [plan, checked, extras, lang]);

  // Едоки показываются, когда семья неоднородная: разные цели, доли или приёмы дома, или есть имена.
  const family = useMemo(() => {
    const m = plan?.members ?? [];
    if (m.length < 2 || plan?.occasion) return null;
    const varied = m.some((x) => x.name || x.factor !== 1 || x.goal !== m[0].goal || x.slots.length !== m[0].slots.length);
    return varied ? m : null;
  }, [plan?.members]);

  if (error) {
    return (
      <div className="shell">
        <TopBar />
        <div className="state">
          <div className="state__box">
            <AlertCircle size={36} aria-hidden />
            <h2>{t("plan.error")}</h2>
            <p>{error}</p>
            <Link to="/" className="btn btn-primary">
              {t("plan.new")}
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (!plan) {
    return (
      <div className="shell">
        <TopBar />
        <div className="receipt" aria-busy="true" aria-label={t("plan.loading")}>
          <div className="receipt__head">
            <div className="skeleton" style={{ height: 28, width: "60%" }} />
            <div className="skeleton" style={{ height: 16, width: "80%" }} />
          </div>
          {Array.from({ length: 4 }, (_, i) => (
            <div key={i} className="day">
              <div className="skeleton" style={{ height: 16, width: 90, marginBottom: 10 }} />
              <div className="skeleton" style={{ height: 44, marginBottom: 6 }} />
              <div className="skeleton" style={{ height: 44 }} />
            </div>
          ))}
        </div>
      </div>
    );
  }

  const dishCount = plan.days.reduce((n, d) => n + d.dishes.filter((x) => !x.leftover).length, 0);
  const cy: Country | undefined = plan.country;
  const rub = (v: number) => money(v, cy, lang);
  const approxRub = (v: number) => approx(v, cy, lang);
  const official = plan.priceSource.coverage > 0 && plan.priceSource.period !== "";

  return (
    <div className={"shell shell--wide" + (storeMode ? " is-store" : "") + (view === "adult" ? " view-adult" : view === "kids" ? " view-kids" : "")}>
      <TopBar
        right={
          storeMode ? (
            <span className="topbar__note">
              <span className="num">
                {bought.n}/{bought.total}
              </span>{" "}
              {t("plan.inCart")}
            </span>
          ) : (
            <span className="topbar__actions">
              {!user && (
                <Link to={`/login?plan=${id}`} className="btn btn-ghost btn-sm" onClick={() => track("plan_save_cta")}>
                  <UserRound size={16} aria-hidden /> {t("plan.save")}
                </Link>
              )}
              <Link to="/?s=1" className="btn btn-ghost btn-sm" onClick={() => track("plan_restart")}>
                <RefreshCw size={16} aria-hidden /> {t("plan.restart")}
              </Link>
            </span>
          )
        }
      />
      {(!online || pending > 0) && (
        <div className={"offline" + (online ? " offline--sync" : "")} role="status">
          <WifiOff size={15} aria-hidden /> {online ? t("offline.pending", { n: pending }) : t("offline.now")}
        </div>
      )}

      <article
        className={"receipt" + (printing.current ? " is-printing" : "") + (pull.dragging ? " is-pulling" : "")}
        style={pull.dragging ? ({ "--pull": `${pull.dy}px` } as CSSProperties) : undefined}
        aria-label={t("plan.aria")}
      >
        <header className="receipt__head">
          <h1 className="receipt__title">{plan.occasion ? `${plan.occasion.title} ${t("occ.for", { n: plan.occasion.guests, guests: tn("guests", plan.occasion.guests) })}` : t("plan.title", { range: weekRange(plan.days[0].date, lang) })}</h1>
          <div className="receipt__meta">
            <span>
              <StoreIcon size={14} aria-hidden /> {plan.store.name}
            </span>
            <span>
              <Users size={14} aria-hidden /> {plan.occasion ? `${plan.occasion.guests} ${tn("guests", plan.occasion.guests)}` : people(plan.params.adults, plan.params.kids?.length ?? 0, lang)}
            </span>
            {plan.goal.level !== "none" && (
              <span>
                <Target size={14} aria-hidden /> {plan.goal.label}
                {plan.goal.kcalTarget > 0 && <> · {plan.goal.kcalTarget} {t("kcal")}</>}
              </span>
            )}
            <span>
              <ScrollText size={14} aria-hidden /> {dishCount} {tn("dishes", dishCount)}
            </span>
          </div>
          {family && (
            <ul className="eaters" aria-label={t("plan.members")}>
              {family.map((m, i) => (
                <li className="eaters__one" key={i}>
                  <b>{m.name || t("quiz.member.name", { n: i + 1 })}</b>
                  <span className="eaters__meta">
                    {m.goal !== "none" && <span>{m.goalLabel}</span>}
                    {m.kcal > 0 && (
                      <span>
                        <span className="num">{m.kcal}</span> {t("plan.member.kcal.unit")}
                      </span>
                    )}
                    {m.factor !== 1 && (
                      <span>
                        {t("plan.member.factor.label")} <span className="num">×{m.factor}</span>
                      </span>
                    )}
                    {m.slots.length < (plan.params.slots?.length ?? 0) && <span>{t("plan.member.slots", { slots: m.slots.map((x) => slotLabel(lang, x).toLowerCase()).join(", ") })}</span>}
                  </span>
                </li>
              ))}
            </ul>
          )}
          <div className="receipt__sums">
            <span>
              <b className="num text-green">{approxRub(plan.totals.cost)}</b> {t("plan.sum.products")}
            </span>
            <span>
              <b className="num">{plan.totals.kcalPerDay}</b> {t(plan.occasion ? "plan.sum.kcal.guest" : "plan.sum.kcal")}
            </span>
            <span>
              <b className="num">{minutes(plan.totals.cookMin)}</b> {t("plan.sum.cook")}
            </span>
          </div>
          {plan.family && plan.family.length > 0 && (
            <p className="eaters__family" aria-label={t("plan.family")}>
              <Users size={13} aria-hidden /> {t("plan.family")}: {plan.family.join(", ")}
            </p>
          )}
          <div className="receipt__source">
            {official ? (
              <>
                <span>
                  {t("plan.src.official", { source: plan.priceSource.name, region: plan.priceSource.region, period: plan.priceSource.period })}
                  {plan.priceSource.weeklyDate && <> {t("plan.src.trend", { date: plan.priceSource.weeklyDate })}</>}
                </span>
                <span>{t("plan.src.index", { store: plan.store.name, pct: Math.round(plan.priceSource.coverage * 100) })}</span>
              </>
            ) : (
              <span>{t("plan.src.estimate", { store: plan.store.name })}</span>
            )}
          </div>
        </header>

        {(plan.warnings.length > 0 || plan.notes.length > 0) && (
          <div className="receipt__warn error-inline" role="status">
            <Info size={18} aria-hidden />
            <span>
              {[...plan.warnings, ...plan.notes].join(" ")}
              {hasLimits && (
                <button type="button" className="btn btn-soft btn-sm receipt__warn-btn" disabled={rebuilding} onClick={rebuildWithoutLimits}>
                  <Unlock size={15} aria-hidden /> {t("plan.limits.clear")}
                </button>
              )}
            </span>
          </div>
        )}

        <div className="receipt__menu">
        {plan.kidsMenus && plan.kidsMenus.length > 0 && (
          <div className="viewswitch" role="radiogroup" aria-label={t("plan.view")}>
            {(["all", "adult", "kids"] as const).map((v) => (
              <button key={v} type="button" role="radio" aria-checked={view === v} onClick={() => setView(v)}>
                {t(`plan.view.${v}`)}
              </button>
            ))}
          </div>
        )}
        {plan.days.map((day) => (
          <section className={"day" + (day.skipped ? " day--away" : "")} key={day.index} aria-label={`${day.label}, ${dateShort(day.date, lang)}`}>
            <div className="day__head">
              <h2 className="day__name">
                {day.label} <small>{dateShort(day.date, lang)}</small>
              </h2>
              <div className="day__sum">
                {day.skipped ? (
                  <span className="day__away">{t("day.skipped")}</span>
                ) : (
                  <>
                    <span className="num">{day.kcal} {t("kcal")}</span>
                    <span className="num">{approxRub(day.cost)}</span>
                  </>
                )}
                {!storeMode && !plan.occasion && (
                  <button type="button" className={"day__skip" + (day.skipped ? " is-on" : "")} onClick={() => skipDay(day.index, !day.skipped)} aria-pressed={!!day.skipped} aria-label={day.skipped ? t("day.unskip") : `${day.label}: ${t("day.skip")}`} title={day.skipped ? t("day.unskip") : t("day.skip.title")}>
                    {day.skipped ? <CalendarPlus size={15} aria-hidden /> : <CalendarOff size={15} aria-hidden />}
                  </button>
                )}
              </div>
            </div>
            {family && day.perMember && (
              <div className="day__members">
                {family.map((m, i) => (
                  <span key={i}>
                    <b>{m.name || t("quiz.member.name", { n: i + 1 })}</b> <span className="num">{Math.round(day.perMember?.[i] ?? 0)}</span> {t("kcal")}
                  </span>
                ))}
              </div>
            )}
            {!day.skipped && day.dishes.map((dish, di) => (
              <DishRow
                key={dish.course ? `${dish.slot}#${di}` : dish.slot}
                dish={dish}
                slotKey={dish.course ? `${dish.slot}#${di}` : dish.slot}
                moving={plan.occasion ? null : moving ? (moving.day === day.index && moving.slot === dish.slot ? "source" : moving.slot === dish.slot && !dish.leftover && !dish.batch ? "target" : "idle") : null}
                onMove={() => (moving && moving.slot === dish.slot && moving.day !== day.index ? move(day.index) : setMoving(moving && moving.day === day.index && moving.slot === dish.slot ? null : { day: day.index, slot: dish.slot }))}
                cy={cy}
                fresh={fresh === `${day.index}:${dish.course ? `${dish.slot}#${di}` : dish.slot}`}
                busy={swapping === `${day.index}:${dish.course ? `${dish.slot}#${di}` : dish.slot}`}
                anyBusy={swapping !== null}
                onOpen={() => {
                  setRecipeId(dish.recipeId);
                  track("recipe_open", { id: dish.recipeId });
                }}
                onSwap={() => swap(day.index, dish.course ? `${dish.slot}#${di}` : dish.slot)}
                onOpenSide={dish.side ? () => { setRecipeId(dish.side!.recipeId); track("recipe_open", { id: dish.side!.recipeId, side: true }); } : undefined}
                onSwapSide={dish.side && !plan.occasion ? () => swapSide(day.index, dish.slot) : undefined}
                onDislike={() => dislike(dish, day.index)}
                ask={!!user && !dish.own && eaten(day.date, dish.slot)}
                onFeedback={(liked) => {
                  api.feedback(dish.recipeId, liked).then(() => setToast(t(liked ? "feedback.thanks.like" : "feedback.thanks.meh"))).catch((e: Error) => setToast(e.message));
                  track("feedback", { id: dish.recipeId, liked });
                }}
              />
            ))}
            {plan.kidsMenus?.map((km) => {
              const kd = km.days[day.index];
              if (!kd || kd.dishes.length === 0) return null;
              return (
                <div className="kidrows" key={km.child}>
                  <div className="kidrows__head">
                    <Baby size={13} aria-hidden /> {t("plan.kid", { age: km.ageLabel })}
                    {day.index === 0 && km.note && <span className="kidrows__note"> · {km.note}</span>}
                  </div>
                  {kd.dishes.map((x) => (
                    <div className="dish dish--kid" key={x.slot}>
                      <div className="dish__slot">{slotLabel(lang, x.slot)}</div>
                      <button
                        type="button"
                        className="dish__main"
                        onClick={() => {
                          setRecipeId(x.recipeId);
                          track("recipe_open", { id: x.recipeId, kid: true });
                        }}
                        aria-label={t("dish.open", { title: x.title })}
                      >
                        <div className="dish__title">{x.title}</div>
                        <div className="dish__why">
                          <span className="dish__slot-inline">{slotLabel(lang, x.slot)}</span>
                          <Clock size={13} aria-hidden /> {x.timeMin} {t("min")}
                        </div>
                      </button>
                      <div className="dish__nums">
                        <b className="num">{approxRub(x.cost)}</b>
                        <span className="num">
                          <Flame size={11} aria-hidden style={{ verticalAlign: -1 }} /> {x.kcal}
                        </span>
                      </div>
                      <span className="dish__actions" aria-hidden />
                    </div>
                  ))}
                </div>
              );
            })}
          </section>
        ))}

        </div>

        <div
          className={"tear" + (pull.dragging ? " is-dragging" : "")}
          role="separator"
          aria-label={t("tear.aria")}
          onPointerDown={onPullStart}
          onPointerMove={onPullMove}
          onPointerUp={onPullEnd}
          onPointerCancel={onPullEnd}
        >
          <i aria-hidden />
          <button type="button" className="tear__btn" onClick={() => setMode(true)}>
            <ShoppingBasket size={14} aria-hidden /> {t("tear.btn")}
          </button>
        </div>

        <aside className="receipt__list">
        <section className="list" aria-label={t("list.title")}>
          <div className="list__head">
            <h2>{t("list.title")}</h2>
            <span>
              {plan.totals.items} {tn("items", plan.totals.items)}
            </span>
          </div>
          {plan.shopping.map((g) => (
            <div className="list__group" key={g.category}>
              <div className="list__cat">
                <span>{g.label}</span>
                {g.cost > 0 && <span className="num">{approxRub(g.cost)}</span>}
              </div>
              {g.items.map((it) => {
                const done = !!checked[it.ingredientId];
                const amount = it.loose || it.unit === "pcs" || it.packs <= 1 ? qty(it.buy, it.unit, lang) : `${it.packs} × ${qty(it.pack, it.unit, lang)}`;
                return (
                  <div key={it.ingredientId} className={"item" + (done ? " is-done" : "") + (it.pantry || it.atHome ? " item--pantry" : "")}>
                    <button
                      type="button"
                      role="checkbox"
                      aria-checked={done}
                      className="item__check"
                      aria-label={`${it.name}: ${done ? t("item.bought") : t("item.notBought")}`}
                      onClick={() => toggleCheck(it.ingredientId, it.name, amount, it.cost)}
                    >
                      <Check size={14} strokeWidth={3} aria-hidden />
                    </button>
                    <div>
                      <div className="item__name">
                        {it.name}
                        <IngredientPic src={it.image} />
                        {it.rosstat && <span className="item__src" title={t("item.official")} aria-label={t("item.official")} />}
                      </div>
                      <div className="item__qty">
                        <span className="num">{it.atHome ? qty(it.needed, it.unit, lang) : amount}</span>
                        {it.atHome ? ` ${t("item.athome")}` : it.home ? ` ${t("item.home", { n: qty(it.home, it.unit, lang) })}` : it.pantry ? ` ${t("item.pantry")}` : ""}
                      </div>
                      <div className="item__used">{it.usedIn.slice(0, 2).join(", ")}</div>
                    </div>
                    <div className="item__cost num">{it.pantry || it.atHome ? "" : approxRub(it.cost)}</div>
                  </div>
                );
              })}
            </div>
          ))}

          <div className="list__group list__group--extras">
            <div className="list__cat">
              <span>{t("list.extras")}</span>
              <span>{extras.length > 0 ? `${extras.length} ${tn("items", extras.length)}` : ""}</span>
            </div>
            {extras.map((e) => {
              const key = `extra:${e.id}`;
              const done = !!checked[key];
              return (
                <div key={e.id} className={"item item--extra" + (done ? " is-done" : "")}>
                  <button type="button" role="checkbox" aria-checked={done} className="item__check" aria-label={`${e.name}: ${done ? t("item.bought") : t("item.notBought")}`} onClick={() => toggleCheck(key, e.name, e.qty, 0)}>
                    <Check size={14} strokeWidth={3} aria-hidden />
                  </button>
                  <div>
                    <div className="item__name">{e.name}</div>
                    <div className="item__qty">
                      {e.qty && <span className="num">{e.qty}</span>}
                      {e.due && <span> {t("extra.by", { date: dateShort(e.due, lang) })}</span>}
                    </div>
                  </div>
                  <button
                    type="button"
                    className="item__del"
                    aria-label={t("extra.remove", { name: e.name })}
                    onClick={async () => {
                      await api.deleteExtra(id, e.id).catch(() => {});
                      setExtras(extras.filter((x) => x.id !== e.id));
                    }}
                  >
                    <Trash2 size={15} aria-hidden />
                  </button>
                </div>
              );
            })}
            {extraForm.open ? (
              <form
                className="extra-form"
                onSubmit={(ev) => {
                  ev.preventDefault();
                  void addExtra();
                }}
              >
                <input className="form-control" placeholder={t("extra.name")} value={extraForm.name} onChange={(e) => setExtraForm({ ...extraForm, name: e.target.value })} maxLength={120} autoFocus />
                <div className="extra-form__row">
                  <input className="form-control" placeholder={t("extra.qty")} value={extraForm.qty} onChange={(e) => setExtraForm({ ...extraForm, qty: e.target.value })} maxLength={40} />
                  <input className="form-control" type="date" value={extraForm.due} onChange={(e) => setExtraForm({ ...extraForm, due: e.target.value })} aria-label={t("extra.due")} />
                </div>
                <div className="extra-form__row">
                  <button type="submit" className="btn btn-primary" disabled={!extraForm.name.trim()}>
                    {t("add")}
                  </button>
                  <button type="button" className="btn btn-ghost" onClick={() => setExtraForm({ open: false, name: "", qty: "", due: "" })}>
                    {t("cancel")}
                  </button>
                </div>
              </form>
            ) : (
              <button type="button" className="btn btn-soft btn-sm extra-add" onClick={() => setExtraForm({ ...extraForm, open: true })}>
                <Plus size={16} aria-hidden /> {t("extra.add")}
              </button>
            )}
          </div>
        </section>

        <footer className="totals">
          <div className="totals__row totals__row--main">
            <span>{t("totals.total")}</span>
            <span className="num">{approxRub(plan.totals.cost)}</span>
          </div>
          <div className="totals__row">
            <span>{t("totals.pantry")}</span>
            <span className="num">+ {approxRub(plan.totals.pantryCost)}</span>
          </div>
          {plan.totals.homeSaved > 0 && (
            <div className="totals__row totals__row--saved">
              <span>{t("totals.home")}</span>
              <span className="num">− {approxRub(plan.totals.homeSaved)}</span>
            </div>
          )}
          {plan.totals.babyCost > 0 && (
            <div className="totals__row">
              <span>{t("totals.baby")}</span>
              <span className="num">{approxRub(plan.totals.babyCost)}</span>
            </div>
          )}
          {plan.totals.kidsMenuCost > 0 && (
            <div className="totals__row">
              <span>{t("totals.kids")}</span>
              <span className="num">{approxRub(plan.totals.kidsMenuCost)}</span>
            </div>
          )}
          <div className="totals__row">
            <span>{t(plan.occasion ? "totals.used.table" : "totals.used")}</span>
            <span className="num">{approxRub(plan.totals.usedCost)}</span>
          </div>
          {plan.totals.cost - plan.totals.babyCost - plan.totals.usedCost > 0 && (
            <div className="totals__row">
              <span>{t(plan.occasion ? "totals.leftover.table" : "totals.leftover")}</span>
              <span className="num">{approxRub(plan.totals.cost - plan.totals.babyCost - plan.totals.usedCost)}</span>
            </div>
          )}
          <div className="totals__row totals__row--goal">
            <span>
              {t("totals.budget", { v: plan.occasion ? t("totals.budget.table") : plan.budget.mode === "week" ? t("totals.budget.week") : t("totals.budget.day", { v: rub(plan.budget.perDay) }) })}
            </span>
            <span className={"num " + (plan.totals.usedCost > plan.budget.targetWeek * 1.05 ? "is-over" : "is-ok")}>
              {rub(plan.budget.targetWeek)} → {approxRub(plan.totals.usedCost)}
            </span>
          </div>
          <div className="totals__row totals__row--stack">
            <span>
              {t(plan.occasion ? "totals.perGuest" : "totals.perDay")}
              {plan.goal.kcalTarget > 0 && <> {t("totals.goal", { n: plan.goal.kcalTarget })}</>}
            </span>
            <span className="num">{t("totals.macros", { kcal: plan.totals.kcalPerDay, p: plan.totals.proteinPerDay, f: plan.totals.fatPerDay, c: plan.totals.carbPerDay })}</span>
          </div>
          <p className="totals__note">
            {official
              ? t("totals.note.official", {
                  source: plan.priceSource.name,
                  region: plan.priceSource.region,
                  period: plan.priceSource.period,
                  trend: plan.priceSource.weeklyDate ? t("totals.note.trend", { date: plan.priceSource.weeklyDate }) : "",
                  store: plan.store.name,
                  idx: plan.store.priceIndex.toFixed(2),
                })
              : t("totals.note.estimate", { store: plan.store.name, idx: plan.store.priceIndex.toFixed(2) })}{" "}
            {t("totals.note.tail")}
          </p>
        </footer>
        </aside>
      </article>

      {moving && (
        <div className="movebar" role="status">
          <ArrowLeftRight size={16} aria-hidden /> {t("dish.move.hint")}
          <button type="button" className="btn btn-link btn-sm" onClick={() => setMoving(null)}>{t("dish.move.cancel")}</button>
        </div>
      )}
      <RecipeSheet recipeId={recipeId} portions={plan.portions} country={plan.country.code} onClose={() => setRecipeId(null)} />
      {user && <PlanChat planId={plan.id} open={chatOpen} onClose={() => setChatOpen(false)} onPlan={(p) => { setPlan(p); setToast(t("chat.applied")); }} />}
      <CartSheet open={cartOpen} onClose={() => setCartOpen(false)} storeCode={plan.store.code} storeName={plan.store.name} country={plan.country.code} items={cartItems} onToast={setToast} />

      <div className="actionbar">
        <div className="actionbar__inner">
          {storeMode ? (
            <>
              <button type="button" className="btn btn-ghost" onClick={() => setMode(false)} aria-label={t("plan.backToMenu")}>
                <ArrowLeft size={20} aria-hidden />
              </button>
              <button type="button" className="btn btn-ghost" onClick={() => setChecked({})} disabled={bought.n === 0} aria-label={t("plan.resetChecks")} title={t("plan.resetChecks")}>
                <RotateCcw size={20} aria-hidden />
              </button>
              <button type="button" className="btn btn-ghost" onClick={share} aria-label={t("plan.share")} title={t("plan.share")}>
                <Share2 size={20} aria-hidden />
              </button>
              <button type="button" className="btn btn-primary" onClick={() => { setCartOpen(true); track("cart_open_sheet"); }}>
                <ShoppingCart size={18} aria-hidden /> {t("cart.button")}
              </button>
            </>
          ) : (
            <>
              {/* Инструменты: на широком экране иконки в ряд, на телефоне прячутся в меню «ещё», чтобы кнопка «В магазин» осталась с текстом */}
              {(() => {
                const tools: { key: string; icon: ReactNode; label: string; title?: string; cls?: string; on: () => void }[] = [
                  { key: "print", icon: <Printer size={20} aria-hidden />, label: t("plan.print"), on: () => { track("plan_print"); window.print(); } },
                ];
                if (user) tools.push({ key: "invite", icon: <Users size={20} aria-hidden />, label: t("plan.family.invite"), on: invite });
                if (user) tools.push({ key: "repeat", icon: <CopyPlus size={20} aria-hidden />, label: t("plan.repeat"), title: t("plan.repeat.title"), on: repeat });
                tools.push({ key: "share", icon: <Share2 size={20} aria-hidden />, label: t("plan.share"), on: share });
                if (user && aiOn) tools.push({ key: "ai", icon: <Sparkles size={20} aria-hidden />, label: t("chat.title"), cls: "actionbar__ai", on: () => { setChatOpen(true); track("plan_chat_open"); } });
                return (
                  <>
                    <div className="actionbar__tools">
                      {tools.map((x) => (
                        <button key={x.key} type="button" className={"btn btn-ghost" + (x.cls ? " " + x.cls : "")} onClick={x.on} aria-label={x.label} title={x.title ?? x.label}>
                          {x.icon}
                        </button>
                      ))}
                    </div>
                    <div className="actionbar__more" ref={moreRef}>
                      <button type="button" className="btn btn-ghost" onClick={() => setMoreOpen((v) => !v)} aria-label={t("more")} title={t("more")} aria-expanded={moreOpen} aria-haspopup="menu">
                        <MoreHorizontal size={20} aria-hidden />
                      </button>
                      {moreOpen && (
                        <div className="actionmenu" role="menu">
                          {tools.map((x) => (
                            <button key={x.key} type="button" role="menuitem" className={"actionmenu__item" + (x.cls ? " " + x.cls : "")} onClick={() => { setMoreOpen(false); x.on(); }}>
                              {x.icon} {x.label}
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  </>
                );
              })()}
              <button type="button" className="btn btn-primary" onClick={() => setMode(true)}>
                <ShoppingBasket size={18} aria-hidden /> {t("plan.toStore")}
              </button>
            </>
          )}
        </div>
      </div>

      {!storeMode && <SiteFooter />}
      {toast && (
        <div className="toast" role="status">
          {toast}
        </div>
      )}
      {!storeMode && <button type="button" className="visually-hidden" onClick={() => nav("/")}>{t("home")}</button>}
    </div>
  );
}

// Прошедший день (или сегодняшний ужин после 19:00): вместо «заменить» спрашиваем «как было?»
function eaten(date: string, slot: string): boolean {
  const now = new Date();
  const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
  if (date < today) return true;
  if (date > today) return false;
  const h = now.getHours();
  return slot === "breakfast" ? h >= 11 : slot === "lunch" ? h >= 16 : h >= 19;
}

function DishRow({ dish, cy, fresh, busy, anyBusy, onOpen, onSwap, onOpenSide, onSwapSide, onDislike, ask, onFeedback, moving, onMove, slotKey: _slotKey }: { dish: Dish; cy: Country | undefined; fresh: boolean; busy: boolean; anyBusy: boolean; onOpen: () => void; onSwap: () => void; onOpenSide?: () => void; onSwapSide?: () => void; onDislike: () => void; ask?: boolean; onFeedback?: (liked: boolean) => void; moving?: "source" | "target" | "idle" | null; onMove?: () => void; slotKey?: string }) {
  const { t, lang } = useT();
  const [answered, setAnswered] = useState<boolean | null>(null);
  return (
    <div className={"dish" + (dish.leftover ? " dish--leftover" : "") + (fresh ? " is-fresh" : "") + (moving === "source" ? " is-moving" : moving === "target" ? " is-target" : "")}>
      <div className="dish__slot">{dish.course ? dish.why : slotLabel(lang, dish.slot)}</div>
      <button type="button" className="dish__main" onClick={onOpen} aria-label={t("dish.open", { title: dish.title })}>
        <div className="dish__title">
          {dish.title}
          {dish.own && <span className="dish__own">{t("own.badge")}</span>}
        </div>
        <div className="dish__why">
          <span className="dish__slot-inline">{dish.course ? dish.why : slotLabel(lang, dish.slot)}</span>
          {dish.leftover ? (
            <>
              <Repeat2 size={13} aria-hidden /> {t("dish.leftover")}
            </>
          ) : (
            <>
              <Clock size={13} aria-hidden /> {dish.timeMin} {t("min")}
              {dish.why && !dish.course && <span className="dish__reason" aria-label={t("dish.why")}>— {dish.why}</span>}
            </>
          )}
        </div>
      </button>
      <div className="dish__nums">
        <b className="num">{approx(dish.cost, cy, lang)}</b>
        <span className="num">
          <Flame size={11} aria-hidden style={{ verticalAlign: -1 }} /> {dish.kcal}
        </span>
      </div>
      {dish.leftover ? (
        <span className="dish__actions" aria-hidden />
      ) : moving === "target" && onMove ? (
        <span className="dish__actions">
          <button type="button" className="dish__swap dish__here" onClick={onMove} aria-label={`${t("dish.move.here")}: ${dish.title}`} title={t("dish.move.here")}>
            <ArrowLeftRight size={16} aria-hidden />
          </button>
        </span>
      ) : ask && onFeedback ? (
        <span className="dish__actions dish__ask" role="group" aria-label={t("feedback.ask")}>
          <button type="button" className={"dish__swap dish__swap--yes" + (answered === true ? " is-on" : "")} onClick={() => { setAnswered(true); onFeedback(true); }} aria-pressed={answered === true} aria-label={`${dish.title}: ${t("feedback.like")}`} title={t("feedback.like")}>
            <ThumbsUp size={15} aria-hidden />
          </button>
          <button type="button" className={"dish__swap dish__swap--no" + (answered === false ? " is-on" : "")} onClick={() => { setAnswered(false); onFeedback(false); }} aria-pressed={answered === false} aria-label={`${dish.title}: ${t("feedback.meh")}`} title={t("feedback.meh")}>
            <ThumbsDown size={15} aria-hidden />
          </button>
        </span>
      ) : (
        <span className="dish__actions">
          <button type="button" className={"dish__swap" + (busy ? " is-busy" : "")} onClick={onSwap} disabled={anyBusy} aria-label={t("dish.swap", { title: dish.title })} title={t("dish.swap.title")}>
            <RefreshCw size={16} aria-hidden />
          </button>
          {!dish.batch && onMove && (
            <button type="button" className={"dish__swap dish__swap--move" + (moving === "source" ? " is-on" : "")} onClick={onMove} disabled={anyBusy} aria-pressed={moving === "source"} aria-label={moving === "source" ? t("dish.move.cancel") : t("dish.move", { title: dish.title })} title={moving === "source" ? t("dish.move.cancel") : t("dish.move.title")}>
              <ArrowLeftRight size={15} aria-hidden />
            </button>
          )}
          {!dish.own && (
            <button type="button" className="dish__swap dish__swap--no" onClick={onDislike} disabled={anyBusy} aria-label={t("dish.dislike", { title: dish.title })} title={t("dish.dislike.title")}>
              <ThumbsDown size={15} aria-hidden />
            </button>
          )}
        </span>
      )}
      {dish.side && (
        <div className="dish__side">
          <button type="button" className="dish__side-open" onClick={onOpenSide} aria-label={t("dish.open", { title: dish.side.title })}>
            <span className="dish__side-tag">{t("dish.side")}</span> {dish.side.title}
            <span className="num dish__side-num">{dish.side.kcal} {t("kcal")}</span>
          </button>
          {onSwapSide && (
            <button type="button" className="dish__side-swap" onClick={onSwapSide} disabled={anyBusy} aria-label={t("dish.side.swap")} title={t("dish.side.swap")}>
              <RefreshCw size={13} aria-hidden />
            </button>
          )}
        </div>
      )}
    </div>
  );
}
