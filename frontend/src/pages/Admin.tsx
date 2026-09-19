import { useEffect, useState } from "react";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { Activity, AlertTriangle, BookOpen, FolderOpen, KeyRound, ScrollText, ShieldCheck, Store, Users } from "lucide-react";
import { AdminRecipes } from "../components/AdminRecipes";
import { AdminModeration } from "../components/AdminModeration";
import { AdminCollections } from "../components/AdminCollections";
import { AdminPartners } from "../components/AdminPartners";
import { AdminOffers } from "../components/AdminOffers";
import { AdminApi } from "../components/AdminApi";
import { EmptyState } from "../components/EmptyState";
import { ABar, AList, ARow, Monogram } from "../components/AdminList";
import { TopBar } from "../components/TopBar";
import { Select } from "../components/Select";
import { BarChart } from "../components/BarChart";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { dateShort } from "../lib/format";
import type { AdminError, AdminLog, AdminOverview, AdminUser, Meta } from "../lib/types";
import { useT } from "../i18n";

// Панель администратора: числа, динамика по дням, события аналитики, аккаунты, ошибки браузера и лог сервера.
// Открыта только почтам из ADMIN_EMAILS; остальным сервер отвечает 404, страница уводит на главную.

type Tab = "overview" | "recipes" | "collections" | "moderation" | "users" | "errors" | "logs" | "partners" | "api";

export function Admin() {
  const { t, lang } = useT();
  const { user, admin, perms, loading } = useAuth();
  const can = (p: string) => perms.includes(p);
  // вкладка и открытый рецепт живут в адресе: /admin/users, /admin/recipes/olivier — работают «назад» и обновление
  const nav = useNavigate();
  const params = useParams<{ tab?: string; id?: string }>();
  const TABS: Tab[] = ["overview", "recipes", "collections", "moderation", "users", "errors", "logs", "partners", "api"];
  const tab: Tab = params.id ? "recipes" : TABS.includes(params.tab as Tab) ? (params.tab as Tab) : "overview";
  const setTab = (next: Tab) => nav(next === "overview" ? "/admin" : `/admin/${next}`);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  useEffect(() => {
    if (!toast) return;
    const id = window.setTimeout(() => setToast(null), 2500);
    return () => window.clearTimeout(id);
  }, [toast]);
  useEffect(() => {
    if (admin) api.meta("RU").then(setMeta).catch(() => setMeta(null));
  }, [admin]);
  const [days, setDays] = useState("14");
  const [ov, setOv] = useState<AdminOverview | null>(null);
  const [ai, setAI] = useState<{ name: string; model: string; today: number; perDay: number; ok: number; failed: number; resting: boolean }[]>([]);
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [errors, setErrors] = useState<AdminError[] | null>(null);
  const [logs, setLogs] = useState<AdminLog[] | null>(null);
  const [level, setLevel] = useState("info");
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!admin) return;
    api.adminOverview(Number(days)).then(setOv).catch((e: Error) => setErr(e.message));
    api.adminAI().then((r) => setAI(r.providers)).catch(() => setAI([]));
  }, [admin, days]);
  useEffect(() => {
    if (!admin) return;
    if (tab === "users" && users === null) api.adminUsers().then(setUsers).catch((e: Error) => setErr(e.message));
    if (tab === "errors" && errors === null) api.adminErrors().then(setErrors).catch((e: Error) => setErr(e.message));
  }, [admin, tab, users, errors]);
  useEffect(() => {
    if (!admin || tab !== "logs") return;
    api.adminLogs(level).then(setLogs).catch((e: Error) => setErr(e.message));
  }, [admin, tab, level]);

  if (loading) return null;
  if (!user || !admin) return <Navigate to="/" replace />;

  const cards: [string, number | undefined, string?][] = [
    [t("admin.users"), ov?.counters.users, t("admin.week", { n: ov?.counters.usersWeek ?? 0 })],
    [t("admin.active"), ov?.counters.activeWeek],
    [t("admin.plans"), ov?.counters.plans, t("admin.week", { n: ov?.counters.plansWeek ?? 0 })],
    [t("admin.plans.owned"), ov?.counters.plansOwned],
    [t("admin.own"), ov?.counters.ownRecipes],
    [t("admin.households"), ov?.counters.households],
    [t("admin.push"), ov?.counters.pushUsers],
    [t("admin.comments"), ov?.counters.comments],
    [t("admin.feedback"), ov?.counters.feedback],
    [t("admin.purchases"), ov?.counters.purchasesWeek],
    [t("admin.errors"), ov?.counters.errorsWeek],
  ];

  return (
    <div className="shell shell--wide">
      <TopBar />
      <main className="admin">
        <h1 className="admin__title">{t("admin.title")}</h1>
        <div className="admin__tabs chips" role="tablist">
          {can("stats") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "overview"} aria-selected={tab === "overview"} onClick={() => setTab("overview")}>
              <Activity size={15} aria-hidden /> {t("admin.tab.overview")}
            </button>
          )}
          {can("recipes") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "recipes"} aria-selected={tab === "recipes"} onClick={() => setTab("recipes")}>
              <BookOpen size={15} aria-hidden /> {t("admin.tab.recipes")}
            </button>
          )}
          {can("recipes") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "collections"} aria-selected={tab === "collections"} onClick={() => setTab("collections")}>
              <FolderOpen size={15} aria-hidden /> {t("admin.tab.collections")}
            </button>
          )}
          {can("moderation") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "moderation"} aria-selected={tab === "moderation"} onClick={() => setTab("moderation")}>
              <ShieldCheck size={15} aria-hidden /> {t("admin.tab.moderation")}
            </button>
          )}
          {can("users") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "users"} aria-selected={tab === "users"} onClick={() => setTab("users")}>
              <Users size={15} aria-hidden /> {t("admin.tab.users")}
            </button>
          )}
          {can("errors") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "errors"} aria-selected={tab === "errors"} onClick={() => setTab("errors")}>
              <AlertTriangle size={15} aria-hidden /> {t("admin.tab.errors")}
            </button>
          )}
          {can("logs") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "logs"} aria-selected={tab === "logs"} onClick={() => setTab("logs")}>
              <ScrollText size={15} aria-hidden /> {t("admin.tab.logs")}
            </button>
          )}
          {can("partners") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "partners"} aria-selected={tab === "partners"} onClick={() => setTab("partners")}>
              <Store size={15} aria-hidden /> {t("admin.partners")}
            </button>
          )}
          {can("recipes") && (
            <button type="button" role="tab" className="chip" aria-pressed={tab === "api"} aria-selected={tab === "api"} onClick={() => setTab("api")}>
              <KeyRound size={15} aria-hidden /> API
            </button>
          )}
        </div>
        {toast && (
          <div className="toast" role="status">
            {toast}
          </div>
        )}
        {tab === "recipes" && can("recipes") && <AdminRecipes equipment={meta?.equipment ?? []} photos={!!meta?.photos} ai={!!meta?.ai} onToast={setToast} editId={params.id} onOpen={(id) => nav(`/admin/recipes/${encodeURIComponent(id)}`)} onClose={() => nav("/admin/recipes")} />}
        {tab === "moderation" && can("moderation") && <AdminModeration onToast={setToast} />}
        {tab === "collections" && can("recipes") && <AdminCollections photos={!!meta?.photos} onToast={setToast} />}
        {tab === "partners" && can("partners") && (
          <>
            <AdminPartners onToast={setToast} />
            <AdminOffers onToast={setToast} />
          </>
        )}
        {tab === "api" && can("recipes") && <AdminApi onToast={setToast} />}
        {err && <p className="error-inline">{err}</p>}

        {tab === "overview" && (
          <section className="admin__section">
            <div className="admin__cards">
              {cards.map(([label, value, sub]) => (
                <div className="admin__card" key={label}>
                  <span className="admin__card-label">{label}</span>
                  <b className="admin__card-value num">{value ?? "…"}</b>
                  {sub && <small>{sub}</small>}
                </div>
              ))}
            </div>
            <div className="admin__row">
              <h2 className="admin__h2">{t("admin.daily")}</h2>
              <Select aria-label={t("admin.period")} value={days} onChange={setDays} options={[{ value: "7", label: t("admin.days", { n: 7 }) }, { value: "14", label: t("admin.days", { n: 14 }) }, { value: "30", label: t("admin.days", { n: 30 }) }, { value: "90", label: t("admin.days", { n: 90 }) }]} />
            </div>
            {ov && (
              <BarChart
                labels={ov.daily.map((d) => dateShort(d.day, lang))}
                series={[
                  { label: t("admin.visitors"), values: ov.daily.map((d) => d.visitors), color: "fill" },
                  { label: t("admin.plans"), values: ov.daily.map((d) => d.plans), color: "blue" },
                  { label: t("admin.users"), values: ov.daily.map((d) => d.users), color: "green" },
                ]}
                format={(v) => String(Math.round(v))}
                height={200}
              />
            )}
            <p className="admin__legend">
              <i className="admin__dot admin__dot--visitors" /> {t("admin.visitors")} <i className="admin__dot admin__dot--plans" /> {t("admin.plans")} <i className="admin__dot admin__dot--users" /> {t("admin.users")}
            </p>
            <div className="admin__grid">
              <div className="admin__panel">
                <h2 className="admin__h2">{t("admin.events")}</h2>
                <p className="admin__hint">{t("admin.events.hint")}</p>
                {ov?.events.map((e) => (
                  <ABar key={e.name} label={e.name} value={e.count} max={ov.events[0]?.count ?? 1} hint={t("admin.sessions.n", { n: e.sessions })} />
                ))}
              </div>
              <div className="admin__panel">
                <h2 className="admin__h2">{t("admin.top.stores")}</h2>
                {ov?.top.stores?.map((x) => (
                  <ABar key={x.key} label={x.key} value={x.count} max={ov.top.stores?.[0]?.count ?? 1} />
                ))}
                <h2 className="admin__h2">{t("admin.top.recipes")}</h2>
                {ov?.top.recipes?.map((x) => (
                  <ABar key={x.key} label={x.key} value={x.count} max={ov.top.recipes?.[0]?.count ?? 1} href={`/recipe/${encodeURIComponent(x.key)}`} />
                ))}
                <h2 className="admin__h2">{t("admin.ai")}</h2>
                <p className="admin__hint">{t("admin.ai.hint")}</p>
                {ai.length === 0 && <p className="admin__hint">{t("admin.ai.none")}</p>}
                {ai.map((p) => (
                  <ABar
                    key={p.name}
                    label={<>{p.name} <small>{p.model}</small>{p.resting && <span className="badge badge--moderator">{t("admin.ai.resting")}</span>}</>}
                    value={p.today}
                    max={p.perDay || Math.max(p.today, 1)}
                    hint={p.perDay ? t("admin.ai.quota", { n: p.today, max: p.perDay }) : t("admin.ai.today", { n: p.today, ok: p.ok, failed: p.failed })}
                  />
                ))}
              </div>
            </div>
          </section>
        )}

        {tab === "users" && (
          <section className="admin__section">
            <p className="admin__hint">{t("admin.users.hint")}</p>
            <AList empty={users?.length === 0 && <EmptyState icon={<Users size={20} />} text={t("admin.logs.empty")} />}>
              {users?.map((u) => (
                <ARow
                  key={u.id}
                  media={<Monogram text={u.name || u.email} />}
                  title={
                    <>
                      {u.name || u.email.split("@")[0]}
                      {u.role && <span className={"badge badge--" + u.role}>{u.role === "admin" ? t("admin.role.admin") : t("admin.role.moderator")}</span>}
                    </>
                  }
                  meta={`${u.email} · ${t("admin.joined")} ${dateShort(u.createdAt.slice(0, 10), lang)}${u.lastSeen ? ` · ${t("admin.seen")} ${dateShort(u.lastSeen.slice(0, 10), lang)}` : ""}`}
                  right={
                    <span className="alist__num">
                      <b className="num">{u.plans}</b>
                      <small>{t("admin.plans").toLowerCase()}</small>
                    </span>
                  }
                  actions={
                    can("roles") &&
                    (u.id === user.id ? (
                      <span className="admin__me">{t("admin.role.you")}</span>
                    ) : (
                      <Select
                        aria-label={t("admin.role")}
                        value={u.role}
                        onChange={(v) => {
                          api.adminSetRole(u.id, v).then(() => { setUsers((users ?? []).map((x) => (x.id === u.id ? { ...x, role: v } : x))); setToast(t("admin.role.saved")); }).catch((e: Error) => setToast(e.message));
                        }}
                        options={[{ value: "", label: t("admin.role.user") }, { value: "moderator", label: t("admin.role.moderator") }, { value: "admin", label: t("admin.role.admin") }]}
                      />
                    ))
                  }
                />
              ))}
            </AList>
          </section>
        )}

        {tab === "errors" && (
          <section className="admin__section">
            {errors?.length === 0 && <EmptyState icon={<AlertTriangle size={20} />} text={t("admin.errors.empty")} />}
            {errors?.map((e, i) => (
              <details className="admin__err" key={i}>
                <summary>
                  <span className="admin__err-time">{e.at.slice(0, 16).replace("T", " ")}</span> <b>{e.message || "—"}</b> <small>{e.url}</small>
                </summary>
                <pre>{e.stack || "—"}</pre>
                <small>{e.ua}</small>
              </details>
            ))}
          </section>
        )}

        {tab === "logs" && (
          <section className="admin__section">
            <div className="admin__row">
              <h2 className="admin__h2">{t("admin.tab.logs")}</h2>
              <Select aria-label={t("admin.level")} value={level} onChange={setLevel} options={[{ value: "info", label: "info+" }, { value: "warn", label: "warn+" }, { value: "error", label: "error" }]} />
            </div>
            <div className="admin__logs">
              {logs?.map((l, i) => (
                <div className={"admin__log admin__log--" + l.level} key={i}>
                  <span className="admin__log-time">{l.time.slice(11, 19)}</span>
                  <span className="admin__log-level">{l.level}</span>
                  <span className="admin__log-msg">
                    {l.logger ? `${l.logger}: ` : ""}
                    {l.msg}
                  </span>
                  {l.fields && Object.keys(l.fields).length > 0 && <code className="admin__log-fields">{JSON.stringify(l.fields)}</code>}
                </div>
              ))}
              {logs?.length === 0 && <EmptyState icon={<ScrollText size={20} />} text={t("admin.logs.empty")} />}
            </div>
          </section>
        )}
      </main>
    </div>
  );
}
