import { useEffect, useState } from "react";
import { Check, ShieldCheck, X } from "lucide-react";
import { EmptyState } from "./EmptyState";
import { api } from "../lib/api";
import type { ModerationItem } from "../lib/types";
import { slotLabel } from "../lib/types";
import { useT } from "../i18n";
import { AList, ARow } from "./AdminList";

// Очередь модерации своих рецептов: что не прошло автопроверку (с причиной нейросети), кнопки
// «опубликовать» / «отклонить» (с причиной для автора); ниже — недавно опубликованные.

export function AdminModeration({ onToast }: { onToast: (s: string) => void }) {
  const { t, lang } = useT();
  const [queue, setQueue] = useState<ModerationItem[] | null>(null);
  const [recent, setRecent] = useState<ModerationItem[]>([]);
  const [note, setNote] = useState<Record<string, string>>({});
  const load = () => api.adminModeration().then((r) => { setQueue(r.queue); setRecent(r.recent); }).catch((e: Error) => onToast(e.message));
  useEffect(() => {
    void load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const decide = async (id: string, approve: boolean, text?: string) => {
    try {
      await api.adminDecide(id, approve, text ?? note[id] ?? "");
      onToast(approve ? t("admin.mod.approved") : t("admin.mod.rejected"));
      void load();
    } catch (e) {
      onToast((e as Error).message);
    }
  };
  const age = (iso?: string) => {
    if (!iso) return "";
    const h = Math.round((Date.now() - new Date(iso).getTime()) / 3600000);
    return h < 1 ? t("admin.mod.justnow") : t("admin.mod.hours", { n: h });
  };

  return (
    <section className="admin__section">
      <h2 className="admin__h2">{t("admin.mod.queue")} {queue && <span className="comments__n num">{queue.length}</span>}</h2>
      {queue === null && <div className="skeleton" style={{ height: 80 }} />}
      {queue?.length === 0 && <EmptyState icon={<ShieldCheck size={20} />} text={t("admin.mod.empty")} />}
      {queue?.map((r) => (
        <article className={"modcard" + (r.submittedAt && Date.now() - new Date(r.submittedAt).getTime() > 20 * 3600000 ? " is-late" : "")} key={r.id}>
          <header className="modcard__head">
            <div>
              <h3 className="modcard__title">
                <a href={`/recipe/${encodeURIComponent(r.id)}`} target="_blank" rel="noopener">
                  {r.title}
                </a>
              </h3>
              <p className="modcard__meta">
                @{r.author || "—"} · {slotLabel(lang, r.slot)} · {r.timeMin} {t("min")} · {age(r.submittedAt)}
              </p>
            </div>
            {r.image && <img className="modcard__img" src={r.image} alt="" />}
          </header>
          {r.note && (
            <p className="modcard__note">
              <b>{t("admin.mod.ai")}:</b> {r.note.startsWith("auto-check") ? t("admin.mod.noai") : r.note}
            </p>
          )}
          {r.description && <p className="modcard__desc">{r.description}</p>}
          <ol className="modcard__steps">
            {r.steps.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ol>
          <p className="modcard__ings">{r.ingredients.map((i) => `${i.ingredientId} ${i.amount}`).join(" · ")}</p>
          <div className="modcard__actions">
            <input className="form-control" value={note[r.id] ?? ""} onChange={(e) => setNote({ ...note, [r.id]: e.target.value })} placeholder={t("admin.mod.note.ph")} aria-label={t("admin.mod.note.ph")} />
            <button type="button" className="btn btn-primary" onClick={() => decide(r.id, true)}>
              <Check size={16} aria-hidden /> {t("admin.mod.approve")}
            </button>
            <button type="button" className="btn btn-danger" onClick={() => decide(r.id, false)}>
              <X size={16} aria-hidden /> {t("admin.mod.reject")}
            </button>
          </div>
        </article>
      ))}
      {recent.length > 0 && (
        <>
          <h2 className="admin__h2">{t("admin.mod.recent")}</h2>
          <AList>
            {recent.map((r) => (
              <ARow
                key={r.id}
                media={r.image ? <img className="alist__thumb" src={r.image} alt="" loading="lazy" /> : <span className={"alist__thumb alist__thumb--" + r.slot} aria-hidden />}
                title={r.title}
                meta={`@${r.author || "—"} · ${slotLabel(lang, r.slot)} · ${r.timeMin} ${t("min")}`}
                href={`/recipe/${encodeURIComponent(r.id)}`}
                actions={
                  <button type="button" className="btn btn-soft btn-sm" onClick={() => void decide(r.id, false, t("admin.mod.unpublish.note"))}>
                    {t("own.unpublish")}
                  </button>
                }
              />
            ))}
          </AList>
        </>
      )}
    </section>
  );
}
