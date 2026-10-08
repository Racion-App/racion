import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { ChevronLeft, ChevronRight, ExternalLink } from "lucide-react";
import { ABar } from "./AdminList";
import { api } from "../lib/api";
import { dateShort } from "../lib/format";
import { intlLocale, useT } from "../i18n";
import type { AdminMetrika as Data, MetrikaRow, MetrikaTotals } from "../lib/types";

// Сводка дня из Яндекс Метрики, чтобы не ходить в саму Метрику: по умолчанию вчера, день листается
// стрелками и живёт в адресе (?d=2026-10-07). Числа сравниваются с днём раньше.

function shift(iso: string, days: number): string {
  const d = new Date(iso + "T12:00:00Z");
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

export function AdminMetrika() {
  const { t, lang } = useT();
  const [params, setParams] = useSearchParams();
  const date = params.get("d") ?? undefined;
  const [data, setData] = useState<Data | null>(null);
  const [err, setErr] = useState<string | null>(null);
  useEffect(() => {
    setErr(null);
    api.adminMetrika(date).then(setData).catch((e: Error) => setErr(e.message));
  }, [date]);

  const nf = new Intl.NumberFormat(intlLocale(lang));
  const nf1 = new Intl.NumberFormat(intlLocale(lang), { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  if (data && !data.enabled) return <p className="admin__hint">{t("admin.metrika.off")}</p>;

  // пока грузится другой день, старые числа не показываем
  const want = date ?? (data?.today ? shift(data.today, -1) : undefined);
  const day = data?.day && (!want || data.day.date === want) ? data.day : undefined;
  const cur = day?.date ?? date;
  const go = (iso: string) => setParams(iso === shift(data?.today ?? iso, -1) ? {} : { d: iso }, { replace: true });
  const title = cur ? new Date(cur + "T00:00:00").toLocaleDateString(intlLocale(lang), { weekday: "long", day: "numeric", month: "long" }) : "…";

  const time = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
  // доля к прошлому дню; для отказов рост — плохо
  const delta = (a: number, b: number, lowerIsBetter = false) => {
    if (!b) return null;
    const p = Math.round(((a - b) / b) * 100);
    const good = p === 0 ? "" : (p > 0) !== lowerIsBetter ? " is-good" : " is-bad";
    return <span className={"admin__delta" + good}>{p > 0 ? "+" : p < 0 ? "−" : "±"}{Math.abs(p)} %</span>;
  };
  const cards: [string, keyof MetrikaTotals, (v: number) => string, boolean?][] = [
    [t("admin.metrika.visits"), "visits", (v) => nf.format(v)],
    [t("admin.metrika.users"), "users", (v) => nf.format(v)],
    [t("admin.metrika.new"), "newUsers", (v) => nf.format(v)],
    [t("admin.metrika.views"), "pageviews", (v) => nf.format(v)],
    [t("admin.metrika.bounce"), "bounce", (v) => nf1.format(v) + " %", true],
    [t("admin.metrika.depth"), "depth", (v) => nf1.format(v)],
    [t("admin.metrika.time"), "duration", time],
  ];

  const total = day?.totals.visits || 1;
  const share = (v: number) => `${Math.round((v / total) * 100)} %`;
  // доля от всех визитов дня — там, где она что-то говорит (источники, устройства); у фраз и городов по 1–2 визита
  const bars = (rows: MetrikaRow[] | undefined, opt: { share?: boolean; href?: boolean } = {}) =>
    !rows ? null : rows.length === 0 ? (
      <p className="admin__hint">{t("admin.metrika.empty")}</p>
    ) : (
      rows.map((r) => <ABar key={r.name} label={r.name} value={r.visits} max={rows[0].visits} hint={opt.share ? share(r.visits) : undefined} href={opt.href && r.name.startsWith("/") && !r.name.includes("…") ? r.name : undefined} />)
    );

  const step = (k: string) => (k === "plan_created" ? 100 : Number(/^quiz_step_(\d+)$/.exec(k)?.[1] ?? 0));
  const funnel = (day?.goals ?? []).filter((g) => step(g.key) > 0).sort((a, b) => step(a.key) - step(b.key));
  const started = funnel[0]?.visits || 1;
  const other = (day?.goals ?? []).filter((g) => step(g.key) === 0 && g.reaches > 0).sort((a, b) => b.reaches - a.reaches);

  return (
    <section className="admin__section">
      <div className="admin__row admin__day">
        <div className="admin__daynav">
          <button type="button" className="admin__navbtn" aria-label={t("admin.metrika.prev")} onClick={() => cur && go(shift(cur, -1))} disabled={!cur}>
            <ChevronLeft size={18} aria-hidden />
          </button>
          <h2 className="admin__h2">{title}</h2>
          <button type="button" className="admin__navbtn" aria-label={t("admin.metrika.next")} onClick={() => cur && go(shift(cur, 1))} disabled={!cur || !data?.today || cur >= data.today}>
            <ChevronRight size={18} aria-hidden />
          </button>
        </div>
        {day && (
          <a className="admin__ext" href={`https://metrika.yandex.ru/dashboard?id=${day.counter}&date1=${day.date}&date2=${day.date}&group=day`} target="_blank" rel="noopener">
            {t("admin.metrika.open")} <ExternalLink size={14} aria-hidden />
          </a>
        )}
      </div>
      {cur && cur === data?.today && <p className="admin__hint">{t("admin.metrika.today")}</p>}
      {err && <p className="error-inline">{err}</p>}

      <div className="admin__cards">
        {cards.map(([label, k, fmt, lower]) => (
          <div className="admin__card" key={k}>
            <span className="admin__card-label">{label}</span>
            <b className="admin__card-value num">{day ? fmt(day.totals[k]) : "…"}</b>
            {day && (
              <small>
                {delta(day.totals[k], day.prevTotals[k], lower)} {dateShort(day.prev, lang)}: {fmt(day.prevTotals[k])}
              </small>
            )}
          </div>
        ))}
      </div>

      <div className="admin__grid">
        <div className="admin__panel">
          <h2 className="admin__h2">{t("admin.metrika.sources")}</h2>
          {bars(day?.sources, { share: true })}
          <h2 className="admin__h2">{t("admin.metrika.engines")}</h2>
          {bars(day?.engines, { share: true })}
          <h2 className="admin__h2">{t("admin.metrika.sites")}</h2>
          {bars(day?.sites)}
        </div>
        <div className="admin__panel">
          <h2 className="admin__h2">{t("admin.metrika.funnel")}</h2>
          <p className="admin__hint">{t("admin.metrika.funnel.hint")}</p>
          {day && funnel.length === 0 && <p className="admin__hint">{t("admin.metrika.empty")}</p>}
          {funnel.map((g) => (
            <ABar key={g.key} label={g.name} value={g.visits} max={started} hint={`${Math.round((g.visits / started) * 100)} %`} />
          ))}
          <h2 className="admin__h2">{t("admin.metrika.goals")}</h2>
          <p className="admin__hint">{t("admin.metrika.goals.hint")}</p>
          {day && other.length === 0 && <p className="admin__hint">{t("admin.metrika.empty")}</p>}
          {other.map((g) => (
            <ABar key={g.key} label={g.name} value={g.reaches} max={other[0].reaches} hint={t("admin.metrika.in.visits", { n: g.visits })} />
          ))}
        </div>
        <div className="admin__panel">
          <h2 className="admin__h2">{t("admin.metrika.pages")}</h2>
          {bars(day?.pages, { share: true, href: true })}
          <h2 className="admin__h2">{t("admin.metrika.devices")}</h2>
          {bars(day?.devices, { share: true })}
        </div>
        <div className="admin__panel">
          <h2 className="admin__h2">{t("admin.metrika.phrases")}</h2>
          {bars(day?.phrases)}
          <h2 className="admin__h2">{t("admin.metrika.cities")}</h2>
          {bars(day?.cities)}
        </div>
      </div>
    </section>
  );
}
