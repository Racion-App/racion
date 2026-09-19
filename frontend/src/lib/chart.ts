// Графики на Chart.js: регистрируем только столбики (tree-shaking), цвета и шрифт берём из токенов
// дизайн-системы в момент отрисовки — так график совпадает с темой, в том числе тёмной.
import { BarController, BarElement, CategoryScale, Chart, Legend, LinearScale, Tooltip, type ChartOptions } from "chart.js";

Chart.register(BarController, BarElement, CategoryScale, LinearScale, Tooltip, Legend);

export { Chart };

export function token(name: string, fallback: string): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

// Общие настройки столбиков: без сетки по X, тонкая сетка по Y, подписи мелкие, тултип в цветах карточки.
export function barOptions(format: (v: number) => string, extra?: { stacked?: boolean }): ChartOptions<"bar"> {
  const text = token("--ds-label-tertiary", "#8e8e93");
  const grid = token("--ds-fill-4", "rgba(120,120,128,0.12)");
  const font = { family: token("--ds-font-sans", "Inter, system-ui, sans-serif"), size: 11 };
  return {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 250 },
    interaction: { mode: "index", intersect: false },
    plugins: {
      legend: { display: false },
      tooltip: {
        backgroundColor: token("--ds-label-primary", "#1c1c1e"),
        titleColor: token("--ds-bg-elevated", "#fff"),
        bodyColor: token("--ds-bg-elevated", "#fff"),
        titleFont: { ...font, size: 12, weight: 600 },
        bodyFont: { ...font, size: 12 },
        padding: 10,
        cornerRadius: 10,
        displayColors: true,
        boxPadding: 4,
        callbacks: { label: (c) => `${c.dataset.label}: ${format(Number(c.parsed.y))}` },
      },
    },
    scales: {
      x: { stacked: !!extra?.stacked, grid: { display: false }, border: { display: false }, ticks: { color: text, font, maxRotation: 0, autoSkip: true, maxTicksLimit: 8 } },
      y: { stacked: !!extra?.stacked, beginAtZero: true, grid: { color: grid }, border: { display: false }, ticks: { color: text, font, maxTicksLimit: 4, callback: (v) => format(Number(v)) } },
    },
  };
}
