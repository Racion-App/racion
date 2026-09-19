import { useEffect, useRef, useState, type FormEvent } from "react";
import { Send, Sparkles } from "lucide-react";
import { api } from "../lib/api";
import type { Plan } from "../lib/types";
import { track } from "../lib/analytics";
import { Sheet } from "./Sheet";
import { useT } from "../i18n";

// Помощник по неделе: чат в листе. Человек пишет «не нравится вторник», сервер отвечает словами и сам
// применяет замены/«не дома»/перестановки — план на чеке обновляется через onPlan.

type Msg = { role: "me" | "ai"; text: string; applied?: string[]; model?: string };

export function PlanChat({ planId, open, onClose, onPlan }: { planId: string; open: boolean; onClose: () => void; onPlan: (p: Plan) => void }) {
  const { t } = useT();
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);
  const quick = [t("chat.q1"), t("chat.q2"), t("chat.q3"), t("chat.q4")];

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight });
  }, [msgs, busy]);

  const send = async (message: string) => {
    const m = message.trim();
    if (!m || busy) return;
    setMsgs((x) => [...x, { role: "me", text: m }]);
    setText("");
    setBusy(true);
    track("plan_chat", { len: m.length });
    try {
      const r = await api.planChat(planId, m);
      setMsgs((x) => [...x, { role: "ai", text: r.reply, applied: r.applied, model: r.model }]);
      if (r.applied.length > 0) onPlan(r.plan);
    } catch (e) {
      setMsgs((x) => [...x, { role: "ai", text: (e as Error).message }]);
    } finally {
      setBusy(false);
    }
  };
  const submit = (e: FormEvent) => {
    e.preventDefault();
    void send(text);
  };

  return (
    <Sheet open={open} onClose={onClose} title={t("chat.title")} closeLabel={t("close")}>
      <h2>{t("chat.title")}</h2>
      <p className="bsheet__lead">{t("chat.lead")}</p>
      <div className="chat" ref={listRef}>
        {msgs.length === 0 && (
          <div className="chat__quick">
            {quick.map((q) => (
              <button key={q} type="button" className="chip" onClick={() => send(q)}>
                {q}
              </button>
            ))}
          </div>
        )}
        {msgs.map((m, i) => (
          <div key={i} className={"chat__msg chat__msg--" + m.role}>
            <p>{m.text}</p>
            {m.applied && m.applied.length > 0 && (
              <ul className="chat__applied">
                {m.applied.map((a, j) => (
                  <li key={j}>{a}</li>
                ))}
              </ul>
            )}
            {m.model && <small className="chat__model">{m.model}</small>}
          </div>
        ))}
        {busy && (
          <div className="chat__msg chat__msg--ai chat__msg--busy" aria-live="polite">
            <Sparkles size={14} className="twinkle" aria-hidden /> {t("chat.thinking")}
          </div>
        )}
      </div>
      <form className="chat__form" onSubmit={submit}>
        <input className="form-control" value={text} onChange={(e) => setText(e.target.value)} placeholder={t("chat.ph")} maxLength={500} aria-label={t("chat.title")} autoComplete="off" />
        <button type="submit" className="btn btn-primary" disabled={busy || !text.trim()} aria-label={t("chat.send")}>
          <Send size={18} aria-hidden />
        </button>
      </form>
    </Sheet>
  );
}
