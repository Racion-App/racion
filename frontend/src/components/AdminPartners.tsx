import { useEffect, useState, type FormEvent } from "react";
import { ExternalLink, Plus, Trash2 } from "lucide-react";
import { api, ApiError } from "../lib/api";
import type { Partner } from "../lib/types";
import { AList, ARow } from "./AdminList";
import { useConfirm } from "./Confirm";
import { expand } from "../lib/partners";
import { useT } from "../i18n";

// Партнёрские магазины: список по странам → форма. Дефолты заливает сервер; здесь меняют адрес
// (партнёрские параметры), ставят флаг «партнёрская» и erid, выключают лишнее.

const EMPTY: Partner = { code: "", country: "RU", kind: "goods", name: "", url: "", affiliate: false, erid: "", active: true, priority: 1 };

export function AdminPartners({ onToast }: { onToast: (s: string) => void }) {
  const { t } = useT();
  const confirm = useConfirm();
  const [list, setList] = useState<Partner[] | null>(null);
  const [editing, setEditing] = useState<Partner | "new" | null>(null);
  const load = () => api.adminPartners().then(setList).catch((e: Error) => onToast(e.message));
  useEffect(() => {
    void load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (editing) {
    return <PartnerForm initial={editing === "new" ? null : editing} onSaved={() => { setEditing(null); onToast(t("admin.partners.saved")); void load(); }} onCancel={() => setEditing(null)} />;
  }
  // основные рынки первыми, остальные по алфавиту
  const FIRST = ["RU", "BY", "KZ"];
  const countries = Array.from(new Set((list ?? []).map((p) => p.country))).sort((a, b) => (FIRST.indexOf(a) + 1 || 99) - (FIRST.indexOf(b) + 1 || 99) || a.localeCompare(b));
  return (
    <section className="admin__section">
      <div className="admin__row">
        <p className="admin__hint">{t("admin.partners.lead")}</p>
        <button type="button" className="btn btn-primary" onClick={() => setEditing("new")}>
          <Plus size={16} aria-hidden /> {t("admin.partners.add")}
        </button>
      </div>
      {countries.map((c) => (
        <div key={c} className="admin__group">
          <h3 className="admin__h3">
            <span className={`fi fi-${c.toLowerCase()}`} aria-hidden /> {t("country." + c)}
          </h3>
          <AList>
            {(list ?? []).filter((p) => p.country === c).map((p) => (
              <ARow
                key={p.code}
                title={
                  <>
                    {p.name}
                    <span className="badge">{t("admin.partners.kind." + p.kind)}</span>
                    {p.affiliate && <span className="badge badge--moderator">{t("admin.partners.affiliate")}</span>}
                    {!p.active && <span className="badge badge--muted">{t("admin.coll.draft")}</span>}
                  </>
                }
                meta={p.url.replace(/^https:\/\//, "")}
                onClick={() => setEditing(p)}
                right={
                  <a className="alist__id" href={expand(p.url, "test")} target="_blank" rel="noopener nofollow" aria-label={p.name}>
                    <ExternalLink size={16} aria-hidden />
                  </a>
                }
                actions={
                  <button
                    type="button"
                    className="planrow__del"
                    aria-label={t("delete")}
                    onClick={async () => {
                      if (!(await confirm({ title: `${t("admin.partners.delete")}: ${p.name}?`, ok: t("delete"), danger: true }))) return;
                      try {
                        await api.adminDeletePartner(p.code);
                        void load();
                      } catch (e) {
                        onToast((e as Error).message);
                      }
                    }}
                  >
                    <Trash2 size={16} aria-hidden />
                  </button>
                }
              />
            ))}
          </AList>
        </div>
      ))}
    </section>
  );
}

function PartnerForm({ initial, onSaved, onCancel }: { initial: Partner | null; onSaved: (p: Partner) => void; onCancel: () => void }) {
  const { t } = useT();
  const [p, setP] = useState<Partner>(initial ?? EMPTY);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const set = (patch: Partial<Partner>) => setP({ ...p, ...patch });
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    try {
      onSaved(await api.adminSavePartner(p));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };
  return (
    <form className="ownform" onSubmit={submit}>
      <h2 className="ownform__title">{initial ? p.name : t("admin.partners.add")}</h2>
      <div className="ownform__grid">
        <label className="auth__field">
          <span>ID</span>
          <input className="form-control" value={p.code} onChange={(e) => set({ code: e.target.value })} pattern="[a-z0-9_]{2,40}" placeholder="ru_ym" required disabled={!!initial} />
        </label>
        <label className="auth__field">
          <span>{t("quiz.q1")}</span>
          <input className="form-control" value={p.country} onChange={(e) => set({ country: e.target.value.toUpperCase() })} maxLength={2} pattern="[A-Za-z]{2}" required />
        </label>
      </div>
      <label className="auth__field">
        <span>{t("own.title")}</span>
        <input className="form-control" value={p.name} onChange={(e) => set({ name: e.target.value })} maxLength={40} required autoFocus={!initial} />
      </label>
      <label className="auth__field">
        <span>URL</span>
        <input className="form-control" value={p.url} onChange={(e) => set({ url: e.target.value })} placeholder="https://market.yandex.ru/search?text={q}&clid=…" required inputMode="url" />
      </label>
      <div className="segmented" role="radiogroup">
        {(["goods", "grocery"] as const).map((k) => (
          <button key={k} type="button" role="radio" aria-checked={p.kind === k} onClick={() => set({ kind: k })}>
            {t("admin.partners.kind." + k)}
          </button>
        ))}
      </div>
      <button type="button" className="switch" role="switch" aria-checked={p.affiliate} onClick={() => set({ affiliate: !p.affiliate })}>
        <span className="switch__text">{t("admin.partners.affiliate")}</span>
        <span className="switch__track" aria-hidden><span className="switch__knob" /></span>
      </button>
      {p.affiliate && (
        <label className="auth__field">
          <span>erid</span>
          <input className="form-control" value={p.erid} onChange={(e) => set({ erid: e.target.value })} maxLength={40} placeholder="2VtzqvXXXXX" />
        </label>
      )}
      <div className="ownform__grid">
        <label className="auth__field">
          <span>#</span>
          <input className="form-control" type="number" value={p.priority} onChange={(e) => set({ priority: Number(e.target.value) || 0 })} min={0} max={99} />
        </label>
        <button type="button" className="switch" role="switch" aria-checked={p.active} onClick={() => set({ active: !p.active })}>
          <span className="switch__text">{t("admin.partners.active")}</span>
          <span className="switch__track" aria-hidden><span className="switch__knob" /></span>
        </button>
      </div>
      {error && <p className="error-inline">{error}</p>}
      <div className="ownform__actions">
        <button type="submit" className="btn btn-primary" disabled={saving}>{t("admin.partners.save")}</button>
        <button type="button" className="btn btn-soft" onClick={onCancel}>{t("cancel")}</button>
      </div>
    </form>
  );
}
