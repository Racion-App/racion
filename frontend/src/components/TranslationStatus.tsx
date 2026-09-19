import { useEffect, useState } from "react";
import { AlertCircle, Check, ChevronDown, Languages, Loader2, RefreshCw } from "lucide-react";
import { api } from "../lib/api";
import type { TranslationStatus as Status, TranslationSummary } from "../lib/types";
import { locales, useT } from "../i18n";

// Переводы своего рецепта: строка «Переводы 5 из 14 · сейчас: Español · mistral/ministral-14b» и по клику
// список языков со статусами. Пока что-то переводится — опрашиваем сервер раз в 4 секунды.

const POLL_MS = 4000;

export function TranslationLine({ recipeId, summary, onToast }: { recipeId: string; summary?: TranslationSummary; onToast: (m: string) => void }) {
  const { t } = useT();
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState<Status | null>(null);
  const [sum, setSum] = useState<TranslationSummary | undefined>(summary);
  const [busy, setBusy] = useState(false);
  const langName = (code: string) => locales().find((l) => l.code === code)?.name ?? code;
  const active = !!sum && sum.done + sum.errors < sum.total;

  // сводка обновляется по опросу, пока перевод идёт; детали — только когда раскрыто
  useEffect(() => {
    if (!active && !open) return;
    let alive = true;
    const tick = () =>
      api.ownTranslations(recipeId).then((s) => {
        if (!alive) return;
        setStatus(s);
        const done = s.items.filter((x) => x.status === "done").length;
        const errors = s.items.filter((x) => x.status === "error").length;
        const running = s.items.find((x) => x.status === "running");
        const model = [...s.items].reverse().find((x) => x.model)?.model;
        setSum({ done, total: s.items.length, errors, running: running?.lang, model });
      }).catch(() => {});
    void tick();
    const id = window.setInterval(tick, POLL_MS);
    return () => { alive = false; window.clearInterval(id); };
  }, [recipeId, active, open]);

  const retry = async () => {
    setBusy(true);
    try {
      const s = await api.ownTranslate(recipeId);
      setStatus(s);
      setSum({ done: s.items.filter((x) => x.status === "done").length, total: s.items.length, errors: 0 });
      setOpen(true);
      onToast(t("tr.retry.sent"));
    } catch (e) {
      onToast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  // рецепт добавлен до появления очереди: предложить перевести
  if (!sum || sum.total === 0) {
    return (
      <div className="trline">
        <button type="button" className="trline__sum" disabled={busy} onClick={retry}>
          <Languages size={13} aria-hidden /> <span>{t("tr.start")}</span>
        </button>
      </div>
    );
  }
  return (
    <div className="trline">
      <button type="button" className="trline__sum" onClick={() => setOpen(!open)} aria-expanded={open}>
        {active ? <Loader2 size={13} className="spin" aria-hidden /> : sum.errors > 0 ? <AlertCircle size={13} aria-hidden /> : <Languages size={13} aria-hidden />}
        <span>
          {t("tr.summary", { done: sum.done, total: sum.total })}
          {sum.running && <> · {t("tr.now", { lang: langName(sum.running) })}</>}
          {sum.errors > 0 && <> · {t("tr.errors", { n: sum.errors })}</>}
          {sum.model && <span className="trline__model"> · {sum.model}</span>}
        </span>
        <ChevronDown size={13} aria-hidden style={{ transform: open ? "rotate(180deg)" : undefined }} />
      </button>
      {open && status && (
        <div className="trline__list">
          {status.items.map((x) => (
            <span key={x.lang} className={"trchip trchip--" + x.status} title={x.error || x.model || ""}>
              {x.status === "done" && <Check size={12} aria-hidden />}
              {x.status === "running" && <Loader2 size={12} className="spin" aria-hidden />}
              {x.status === "error" && <AlertCircle size={12} aria-hidden />}
              {langName(x.lang)}
            </span>
          ))}
          <span className="trline__foot">
            {status.providers.length > 0 ? t("tr.providers", { list: status.providers.join(", ") }) : t("tr.providers.none")}
            {(sum.errors > 0 || sum.done < sum.total) && !active && (
              <button type="button" className="btn btn-link btn-sm" disabled={busy} onClick={retry}>
                <RefreshCw size={13} aria-hidden /> {t("tr.retry")}
              </button>
            )}
          </span>
        </div>
      )}
    </div>
  );
}
