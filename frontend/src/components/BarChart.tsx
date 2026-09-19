import { useEffect, useRef } from "react";
import { Chart, barOptions, token } from "../lib/chart";

// Столбики на canvas (Chart.js): адаптивная ширина, тултип по касанию, цвета из темы.
// Перерисовывается при смене темы (атрибут data-theme на <html>) и системной схемы.

export type Series = { label: string; values: number[]; color: "blue" | "green" | "orange" | "fill" };

const COLOR: Record<Series["color"], [string, string]> = {
  blue: ["--ds-blue", "#0a84ff"],
  green: ["--ds-green", "#30d158"],
  orange: ["--ds-orange", "#ff9f0a"],
  fill: ["--ds-fill-2", "rgba(120,120,128,0.32)"],
};

export function BarChart({ labels, series, format, height = 180, stacked }: { labels: string[]; series: Series[]; format: (v: number) => string; height?: number; stacked?: boolean }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    let chart: Chart<"bar"> | null = null;
    const draw = () => {
      chart?.destroy();
      chart = new Chart(el, {
        type: "bar",
        data: {
          labels,
          datasets: series.map((s) => ({ label: s.label, data: s.values, backgroundColor: token(...COLOR[s.color]), borderRadius: 4, borderSkipped: false, maxBarThickness: 22, categoryPercentage: 0.7, barPercentage: 0.9 })),
        },
        options: barOptions(format, { stacked }),
      });
    };
    draw();
    const mo = new MutationObserver(draw);
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    mq.addEventListener("change", draw);
    return () => {
      mo.disconnect();
      mq.removeEventListener("change", draw);
      chart?.destroy();
    };
  }, [labels, series, format, stacked]);
  return (
    <div className="chart" style={{ height }}>
      <canvas ref={ref} />
    </div>
  );
}
