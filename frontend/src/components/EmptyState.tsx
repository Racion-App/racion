import type { ReactNode } from "react";

// Пустое состояние списка: карточка с иконкой, заголовком, одной строкой пояснения и действием.
// Строки локалей уже написаны как «Заголовок. Пояснение.» — делим по первой точке, чтобы не плодить ключи.

export function EmptyState({ icon, text, hint, action }: { icon: ReactNode; text: string; hint?: string; action?: ReactNode }) {
  const [title, rest] = hint ? [text.replace(/[.。]$/, ""), hint] : splitEmpty(text);
  return (
    <div className="empty" role="status">
      <span className="empty__icon" aria-hidden>{icon}</span>
      <p className="empty__title">{title}</p>
      {rest && <p className="empty__hint">{rest}</p>}
      {action && <div className="empty__action">{action}</div>}
    </div>
  );
}

function splitEmpty(s: string): [string, string] {
  const m = /^(.+?[.!?。])\s*(.+)$/.exec(s.trim());
  if (!m) return [s.replace(/[.。]$/, ""), ""];
  return [m[1].replace(/[.。]$/, ""), m[2]];
}
