// Печатающийся чек — состояние «считаем неделю». Строки прорисовываются штрихом,
// лист «выезжает» из прорези; всё на SMIL, чтобы работало и внутри кнопки.
export function ReceiptLoader({ size = 22, label = "" }: { size?: number; label?: string }) {
  const lines = [10, 14, 18, 22];
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" role="img" aria-label={label} fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
      <path d="M5 8h22" />
      <g>
        <path d="M9 4h14v22l-2.3-1.6-2.3 1.6-2.4-1.6-2.4 1.6-2.3-1.6L9 26z" fill="none">
          <animateTransform attributeName="transform" type="translate" values="0 -18; 0 0; 0 0" keyTimes="0; 0.55; 1" dur="1.8s" repeatCount="indefinite" calcMode="spline" keySplines="0.32 0.72 0 1; 0 0 1 1" />
        </path>
        {lines.map((y, i) => (
          <path key={y} d={`M12 ${y}h${i === lines.length - 1 ? 5 : 8}`} strokeDasharray="8" strokeDashoffset="8">
            <animate attributeName="stroke-dashoffset" values="8;0;0" keyTimes="0;0.4;1" dur="1.8s" begin={`${0.55 + i * 0.18}s`} repeatCount="indefinite" />
            <animateTransform attributeName="transform" type="translate" values="0 -18; 0 0; 0 0" keyTimes="0; 0.55; 1" dur="1.8s" repeatCount="indefinite" calcMode="spline" keySplines="0.32 0.72 0 1; 0 0 1 1" />
          </path>
        ))}
      </g>
      <rect x="4" y="6" width="24" height="4" rx="1" fill="var(--ds-bg-base, #F2F2F7)" stroke="none" />
      <path d="M5 8h22" />
    </svg>
  );
}
