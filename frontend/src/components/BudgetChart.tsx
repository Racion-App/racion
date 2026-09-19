import { useCallback, useEffect, useMemo, useState } from "react";
import { TrendingDown, TrendingUp } from "lucide-react";
import { api } from "../lib/api";
import { approx, dateShort, money } from "../lib/format";
import type { BudgetReport, Country } from "../lib/types";
import { useT } from "../i18n";
import { BarChart, type Series } from "./BarChart";

// Динамика бюджета: столбики по неделям «план» (серый) и «куплено» (синий) на Chart.js — на телефоне
// подписи не наезжают, значения смотрятся касанием; под графиком — сравнение среднего дневного расхода
// этого месяца с прошлым.

export function BudgetChart({ country }: { country: Country | undefined }) {
  const { t, lang } = useT();
  const [rep, setRep] = useState<BudgetReport | null | undefined>(undefined);
  useEffect(() => {
    api.budget().then(setRep).catch(() => setRep(null));
  }, []);
  const format = useCallback((v: number) => money(v, country, lang), [country, lang]);
  const labels = useMemo(() => rep?.weeks.map((w) => dateShort(w.start, lang)) ?? [], [rep, lang]);
  const series = useMemo<Series[]>(
    () => (rep ? [{ label: t("budget.planned"), values: rep.weeks.map((w) => w.planned), color: "fill" }, { label: t("budget.bought"), values: rep.weeks.map((w) => w.bought), color: "blue" }] : []),
    [rep, t],
  );
  if (rep === undefined) return <div className="skeleton" style={{ height: 160 }} />;
  if (!rep || rep.weeks.every((w) => w.planned === 0 && w.bought === 0)) return null;
  const fmt = (v: number) => approx(v, country, lang);

  return (
    <section className="budget" aria-label={t("budget.title")}>
      <div className="budget__head">
        <h3 className="budget__title">{t("budget.title")}</h3>
        <span className="budget__legend">
          <i className="budget__dot budget__dot--plan" /> {t("budget.planned")}
          <i className="budget__dot budget__dot--fact" /> {t("budget.bought")}
        </span>
      </div>
      <BarChart labels={labels} series={series} format={format} height={190} />
      <div className="budget__foot">
        {rep.deltaPct !== undefined && rep.deltaPct !== null ? (
          <p className={"budget__delta" + (rep.deltaPct < 0 ? " is-down" : " is-up")}>
            {rep.deltaPct < 0 ? <TrendingDown size={16} aria-hidden /> : <TrendingUp size={16} aria-hidden />}
            {rep.deltaPct < 0 ? t("budget.less", { pct: Math.abs(rep.deltaPct) }) : rep.deltaPct === 0 ? t("budget.same") : t("budget.more", { pct: rep.deltaPct })}
          </p>
        ) : (
          <p className="budget__delta">{t("budget.month", { sum: fmt(rep.months[1]?.bought ?? 0) })}</p>
        )}
      </div>
    </section>
  );
}
