import { useEffect, useState, type ReactElement } from "react";
import { useT } from "../i18n";

// Вход через внешние сервисы. Список даёт сервер: для России VK и Яндекс, остальным все включённые.
// Кнопка — обычная ссылка на /api/auth/oauth/{id}/start: дальше редиректы, React тут не нужен.

type Provider = { id: string; name: string };

const ICONS: Record<string, ReactElement> = {
  vk: (
    <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
      <rect width="24" height="24" rx="6" fill="#0077FF" />
      <path fill="#fff" d="M12.8 17c-4.9 0-7.7-3.4-7.8-9h2.5c.1 4.1 1.9 5.9 3.3 6.2V8h2.3v3.5c1.4-.1 2.9-1.7 3.4-3.5H19c-.4 2.2-2 3.8-3.2 4.5 1.2.5 3 1.9 3.7 4.5h-2.6c-.5-1.7-1.9-3-3.7-3.2V17h-.4z" />
    </svg>
  ),
  yandex: (
    <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
      <rect width="24" height="24" rx="6" fill="#FC3F1D" />
      <path fill="#fff" d="M13.9 19h-2.4v-5.6L7.5 5h2.6l2.7 6.1L15.9 5h2.4l-4.4 8.4V19z" />
    </svg>
  ),
  google: (
    <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
      <path fill="#4285F4" d="M21.6 12.2c0-.7-.1-1.4-.2-2H12v3.9h5.4a4.6 4.6 0 0 1-2 3v2.5h3.2c1.9-1.7 3-4.3 3-7.4z" />
      <path fill="#34A853" d="M12 22c2.7 0 5-.9 6.6-2.4l-3.2-2.5c-.9.6-2 1-3.4 1-2.6 0-4.8-1.8-5.6-4.1H3.1v2.6A10 10 0 0 0 12 22z" />
      <path fill="#FBBC05" d="M6.4 13.9a6 6 0 0 1 0-3.8V7.5H3.1a10 10 0 0 0 0 9l3.3-2.6z" />
      <path fill="#EA4335" d="M12 6c1.5 0 2.8.5 3.8 1.5l2.8-2.8A10 10 0 0 0 3.1 7.5l3.3 2.6C7.2 7.8 9.4 6 12 6z" />
    </svg>
  ),
  github: (
    <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
      <path fill="currentColor" d="M12 1.5a10.5 10.5 0 0 0-3.3 20.5c.5.1.7-.2.7-.5v-1.9c-2.9.6-3.5-1.3-3.5-1.3-.5-1.2-1.2-1.5-1.2-1.5-1-.7.1-.7.1-.7 1.1.1 1.6 1.1 1.6 1.1.9 1.6 2.5 1.2 3.1.9.1-.7.4-1.2.7-1.4-2.3-.3-4.8-1.2-4.8-5.2 0-1.1.4-2.1 1.1-2.8-.1-.3-.5-1.3.1-2.8 0 0 .9-.3 2.9 1.1a10 10 0 0 1 5.3 0c2-1.4 2.9-1.1 2.9-1.1.6 1.5.2 2.5.1 2.8.7.7 1.1 1.7 1.1 2.8 0 4-2.5 4.9-4.8 5.2.4.3.7 1 .7 2v2.9c0 .3.2.6.7.5A10.5 10.5 0 0 0 12 1.5z" />
    </svg>
  ),
  apple: (
    <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
      <path fill="currentColor" d="M16.4 12.7c0-2.4 2-3.6 2.1-3.7-1.1-1.7-2.9-1.9-3.5-1.9-1.5-.2-2.9.9-3.7.9-.8 0-1.9-.9-3.2-.8-1.6 0-3.1 1-4 2.4-1.7 3-.4 7.3 1.2 9.7.8 1.2 1.8 2.5 3 2.4 1.2 0 1.7-.8 3.2-.8s1.9.8 3.2.8c1.3 0 2.2-1.2 3-2.4.9-1.4 1.3-2.7 1.3-2.8 0 0-2.6-1-2.6-3.8zM14 5.5c.7-.8 1.1-1.9 1-3-1 0-2.1.7-2.8 1.5-.6.7-1.2 1.8-1 2.9 1.1.1 2.2-.6 2.8-1.4z" />
    </svg>
  ),
};

export function OAuthButtons({ plan, next }: { plan?: string; next?: string }) {
  const { t } = useT();
  const [list, setList] = useState<Provider[]>([]);
  useEffect(() => {
    let alive = true;
    fetch("/api/auth/providers", { credentials: "same-origin" })
      .then((r) => (r.ok ? r.json() : { providers: [] }))
      .then((d: { providers: Provider[] }) => { if (alive) setList(d.providers ?? []); })
      .catch(() => {});
    return () => { alive = false; };
  }, []);
  if (list.length === 0) return null;
  const q = new URLSearchParams();
  if (plan) q.set("plan", plan);
  if (next) q.set("next", next);
  const qs = q.toString() ? `?${q}` : "";
  return (
    <div className="oauth">
      <div className="oauth__sep"><span>{t("auth.via")}</span></div>
      <div className="oauth__row">
        {list.map((p) => (
          <a key={p.id} className="btn btn-soft oauth__btn" href={`/api/auth/oauth/${p.id}/start${qs}`} aria-label={t("auth.via.one", { name: p.name })}>
            {ICONS[p.id]}
            <span>{p.name}</span>
          </a>
        ))}
      </div>
    </div>
  );
}
