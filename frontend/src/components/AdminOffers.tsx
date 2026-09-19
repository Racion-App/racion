import { useEffect, useState, type FormEvent } from "react";
import { ExternalLink, Plus, Trash2 } from "lucide-react";
import { api, ApiError } from "../lib/api";
import type { Offer } from "../lib/types";
import { AList, ARow } from "./AdminList";
import { useConfirm } from "./Confirm";
import { OfferCard } from "./OfferCard";
import { useT } from "../i18n";
import type { Meta } from "../lib/types";

// Рекламные предложения: конкретный товар или акция магазина. Показываются одним блоком там, где уместны:
// у корзины (промокод на первый заказ), в рецепте (техника или продукт из рецепта), над списком покупок.
// Таргетинг: страна, при желании регионы или города Росстата, даты и приоритет.

const EMPTY: Offer = { id: "", country: "RU", regions: [], place: "cart", match: [], title: "", body: "", cta: "", url: "", image: "", promo: "", affiliate: true, erid: "", startsAt: null, endsAt: null, active: true, priority: 1 };
const PLACES = ["cart", "recipe", "plan"] as const;

export function AdminOffers({ onToast }: { onToast: (s: string) => void }) {
  const { t } = useT();
  const confirm = useConfirm();
  const [list, setList] = useState<Offer[] | null>(null);
  const [editing, setEditing] = useState<Offer | "new" | null>(null);
  const load = () => api.adminOffers().then(setList).catch((e: Error) => onToast(e.message));
  useEffect(() => {
    void load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (editing) {
    return <OfferForm initial={editing === "new" ? null : editing} onSaved={() => { setEditing(null); onToast(t("admin.partners.saved")); void load(); }} onCancel={() => setEditing(null)} />;
  }
  return (
    <section className="admin__section">
      <div className="admin__row">
        <p className="admin__hint">{t("admin.offers.lead")}</p>
        <button type="button" className="btn btn-primary" onClick={() => setEditing("new")}>
          <Plus size={16} aria-hidden /> {t("admin.offers.add")}
        </button>
      </div>
      {list && list.length === 0 && <p className="ownform__empty">{t("admin.offers.empty")}</p>}
      <AList>
        {(list ?? []).map((o) => (
          <ARow
            key={o.id}
            title={
              <>
                <span className={`fi fi-${o.country.toLowerCase()}`} aria-hidden /> {o.title}
                <span className="badge">{t("admin.offers.place." + o.place)}</span>
                {o.regions.length > 0 && <span className="badge badge--moderator">{t("admin.offers.regions.n", { n: o.regions.length })}</span>}
                {!o.active && <span className="badge badge--muted">{t("admin.coll.draft")}</span>}
              </>
            }
            meta={[o.promo && `${t("offer.promo")} ${o.promo}`, o.endsAt && `${t("admin.offers.until")} ${o.endsAt}`, o.url.replace(/^https:\/\//, "")].filter(Boolean).join(" · ")}
            onClick={() => setEditing(o)}
            right={
              <a className="alist__id" href={o.url} target="_blank" rel="noopener nofollow" aria-label={o.title}>
                <ExternalLink size={16} aria-hidden />
              </a>
            }
            actions={
              <button
                type="button"
                className="planrow__del"
                aria-label={t("delete")}
                onClick={async () => {
                  if (!(await confirm({ title: `${t("admin.partners.delete")}: ${o.title}?`, ok: t("delete"), danger: true }))) return;
                  try {
                    await api.adminDeleteOffer(o.id);
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
    </section>
  );
}

function OfferForm({ initial, onSaved, onCancel }: { initial: Offer | null; onSaved: (o: Offer) => void; onCancel: () => void }) {
  const { t } = useT();
  const [o, setO] = useState<Offer>(initial ?? EMPTY);
  const [meta, setMeta] = useState<Meta | null>(null);
  useEffect(() => {
    if (o.country.length === 2) api.meta(o.country).then(setMeta).catch(() => undefined);
  }, [o.country]);
  const [regionQ, setRegionQ] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const set = (patch: Partial<Offer>) => setO({ ...o, ...patch });
  const regions = meta?.regions ?? [];
  const regionName = (code: string) => regions.find((r) => r.code === code)?.name ?? code;
  const q = regionQ.trim().toLowerCase();
  const hits = q ? regions.filter((r) => r.kind !== "rf" && !o.regions.includes(r.code) && r.name.toLowerCase().includes(q)).slice(0, 8) : [];
  const matchOptions = [...(meta?.equipment ?? []), ...(meta?.ingredients ?? [])];
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    try {
      onSaved(await api.adminSaveOffer(o));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };
  return (
    <form className="ownform" onSubmit={submit}>
      <h2 className="ownform__title">{initial ? o.title : t("admin.offers.add")}</h2>
      <div className="ownform__grid">
        <label className="auth__field">
          <span>ID</span>
          <input className="form-control" value={o.id} onChange={(e) => set({ id: e.target.value })} pattern="[a-z0-9][a-z0-9_-]{1,63}" placeholder="ru_5ka_first" required disabled={!!initial} />
        </label>
        <label className="auth__field">
          <span>{t("quiz.q1")}</span>
          <input className="form-control" value={o.country} onChange={(e) => set({ country: e.target.value.toUpperCase() })} maxLength={2} pattern="[A-Za-z]{2}" required />
        </label>
      </div>
      <div className="segmented" role="radiogroup" aria-label={t("admin.offers.place")}>
        {PLACES.map((k) => (
          <button key={k} type="button" role="radio" aria-checked={o.place === k} onClick={() => set({ place: k })}>
            {t("admin.offers.place." + k)}
          </button>
        ))}
      </div>
      <p className="quiz__hint">{t("admin.offers.place.hint." + o.place)}</p>
      <label className="auth__field">
        <span>{t("own.title")}</span>
        <input className="form-control" value={o.title} onChange={(e) => set({ title: e.target.value })} maxLength={80} required autoFocus={!initial} />
      </label>
      <label className="auth__field">
        <span>{t("admin.offers.body")}</span>
        <textarea className="form-control" value={o.body} onChange={(e) => set({ body: e.target.value })} maxLength={300} rows={2} />
      </label>
      <div className="ownform__grid">
        <label className="auth__field">
          <span>{t("admin.offers.cta")}</span>
          <input className="form-control" value={o.cta} onChange={(e) => set({ cta: e.target.value })} maxLength={30} placeholder={t("offer.open")} />
        </label>
        <label className="auth__field">
          <span>{t("admin.offers.promo")}</span>
          <input className="form-control" value={o.promo} onChange={(e) => set({ promo: e.target.value })} maxLength={30} />
        </label>
      </div>
      <label className="auth__field">
        <span>URL</span>
        <input className="form-control" value={o.url} onChange={(e) => set({ url: e.target.value })} placeholder="https://…" required inputMode="url" />
      </label>
      <label className="auth__field">
        <span>{t("admin.offers.image")}</span>
        <input className="form-control" value={o.image} onChange={(e) => set({ image: e.target.value })} placeholder="https://…/product.webp" inputMode="url" />
      </label>
      {o.country === "RU" && (
        <div className="auth__field">
          <span>{t("admin.offers.regions")}</span>
          {o.regions.length > 0 && (
            <div className="chips">
              {o.regions.map((code) => (
                <button key={code} type="button" className="chip chip--remove" onClick={() => set({ regions: o.regions.filter((c) => c !== code) })}>
                  {regionName(code)}
                </button>
              ))}
            </div>
          )}
          <input className="form-control" value={regionQ} onChange={(e) => setRegionQ(e.target.value)} placeholder={t("quiz.region.placeholder")} autoComplete="off" />
          {hits.length > 0 && (
            <div className="suggest" role="listbox">
              {hits.map((r) => (
                <button key={r.code} type="button" role="option" aria-selected={false} onClick={() => { set({ regions: [...o.regions, r.code] }); setRegionQ(""); }}>
                  {r.name}
                  {r.kind === "city" && <span className="suggest__sub"> · {regionName(r.parent)}</span>}
                </button>
              ))}
            </div>
          )}
          <p className="quiz__hint">{t("admin.offers.regions.hint")}</p>
        </div>
      )}
      {o.place === "recipe" && (
        <label className="auth__field">
          <span>{t("admin.offers.match")}</span>
          <input className="form-control" value={o.match.join(", ")} onChange={(e) => set({ match: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) })} placeholder="blender, airfryer, salmon" list="offer-match" />
          <datalist id="offer-match">{matchOptions.map((x) => <option key={x.id} value={x.id}>{x.label}</option>)}</datalist>
          <span className="quiz__hint">{t("admin.offers.match.hint")}</span>
        </label>
      )}
      <div className="ownform__grid">
        <label className="auth__field">
          <span>{t("admin.offers.from")}</span>
          <input className="form-control" type="date" value={o.startsAt ?? ""} onChange={(e) => set({ startsAt: e.target.value || null })} />
        </label>
        <label className="auth__field">
          <span>{t("admin.offers.until")}</span>
          <input className="form-control" type="date" value={o.endsAt ?? ""} onChange={(e) => set({ endsAt: e.target.value || null })} />
        </label>
      </div>
      <button type="button" className="switch" role="switch" aria-checked={o.affiliate} onClick={() => set({ affiliate: !o.affiliate })}>
        <span className="switch__text">{t("admin.partners.affiliate")}</span>
        <span className="switch__track" aria-hidden><span className="switch__knob" /></span>
      </button>
      {o.affiliate && (
        <label className="auth__field">
          <span>erid</span>
          <input className="form-control" value={o.erid} onChange={(e) => set({ erid: e.target.value })} maxLength={40} placeholder="2VtzqvXXXXX" />
        </label>
      )}
      <div className="ownform__grid">
        <label className="auth__field">
          <span>#</span>
          <input className="form-control" type="number" value={o.priority} onChange={(e) => set({ priority: Number(e.target.value) || 0 })} min={0} max={99} />
        </label>
        <button type="button" className="switch" role="switch" aria-checked={o.active} onClick={() => set({ active: !o.active })}>
          <span className="switch__text">{t("admin.partners.active")}</span>
          <span className="switch__track" aria-hidden><span className="switch__knob" /></span>
        </button>
      </div>
      {o.title && o.url && (
        <div className="auth__field">
          <span>{t("admin.offers.preview")}</span>
          <OfferCard offer={o} />
        </div>
      )}
      {error && <p className="error-inline">{error}</p>}
      <div className="ownform__actions">
        <button type="submit" className="btn btn-primary" disabled={saving}>{t("admin.partners.save")}</button>
        <button type="button" className="btn btn-soft" onClick={onCancel}>{t("cancel")}</button>
      </div>
    </form>
  );
}
