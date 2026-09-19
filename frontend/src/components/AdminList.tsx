import type { ReactNode } from "react";

// Списки админки в духе кабинета: карточка-ряд «медиа · заголовок и подпись · справа числа и действия».
// Вместо таблиц: читается на телефоне, у каждой строки есть куда положить фото, чип и кнопку.

export function AList({ children, empty }: { children: ReactNode; empty?: ReactNode }) {
  return (
    <div className="alist">
      {children}
      {empty}
    </div>
  );
}

export function ARow({ media, title, meta, tags, right, actions, onClick, href, late }: { media?: ReactNode; title: ReactNode; meta?: ReactNode; tags?: ReactNode; right?: ReactNode; actions?: ReactNode; onClick?: () => void; href?: string; late?: boolean }) {
  const body = (
    <>
      <span className="alist__title">{title}</span>
      {meta && <span className="alist__meta">{meta}</span>}
      {tags && <span className="alist__tags">{tags}</span>}
    </>
  );
  return (
    <div className={"alist__row" + (late ? " is-late" : "")}>
      {media && <span className="alist__media">{media}</span>}
      {onClick ? (
        <button type="button" className="alist__main alist__main--btn" onClick={onClick}>
          {body}
        </button>
      ) : href ? (
        <a className="alist__main" href={href} target="_blank" rel="noopener">
          {body}
        </a>
      ) : (
        <span className="alist__main">{body}</span>
      )}
      {right && <span className="alist__right">{right}</span>}
      {actions && <span className="alist__actions">{actions}</span>}
    </div>
  );
}

// Монограмма для аккаунта без фото.
export function Monogram({ text, src }: { text: string; src?: string }) {
  return src ? <img className="alist__ava" src={src} alt="" /> : <span className="alist__ava alist__ava--mono">{(text || "?").slice(0, 1).toUpperCase()}</span>;
}

// Строка «подпись · число · полоска доли» для событий и топов.
export function ABar({ label, value, max, hint, href }: { label: ReactNode; value: number; max: number; hint?: ReactNode; href?: string }) {
  const pct = max > 0 ? Math.max(2, Math.round((value / max) * 100)) : 0;
  return (
    <div className="abar">
      <span className="abar__label">{href ? <a href={href}>{label}</a> : label}</span>
      <span className="abar__value num">{value}</span>
      {hint && <span className="abar__hint">{hint}</span>}
      <span className="abar__track" aria-hidden>
        <span className="abar__fill" style={{ width: `${pct}%` }} />
      </span>
    </div>
  );
}
