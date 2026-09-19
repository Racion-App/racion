import { useEffect, useState, type FormEvent } from "react";
import { Copy, KeyRound, Plus, Trash2 } from "lucide-react";
import { api } from "../lib/api";
import type { ApiKey } from "../lib/types";
import { useConfirm } from "./Confirm";
import { dateShort } from "../lib/format";
import { useT } from "../i18n";

// Ключи API для скриптов и помощников (Claude, Codex): создать → ключ показывается один раз → скопировать.
// Список показывает только префикс и дату последнего использования.

export function AdminKeys({ onToast }: { onToast: (s: string) => void }) {
  const { t, lang } = useT();
  const confirm = useConfirm();
  const [keys, setKeys] = useState<ApiKey[] | null>(null);
  const [name, setName] = useState("");
  const [fresh, setFresh] = useState<ApiKey | null>(null);
  const load = () => api.apiKeys().then(setKeys).catch((e: Error) => onToast(e.message));
  useEffect(() => {
    void load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const create = async (e: FormEvent) => {
    e.preventDefault();
    try {
      const k = await api.apiKeyCreate(name.trim() || "Claude");
      setFresh(k);
      setName("");
      void load();
    } catch (err) {
      onToast((err as Error).message);
    }
  };
  const copy = async (s: string) => {
    try {
      await navigator.clipboard.writeText(s);
      onToast(t("admin.keys.copied"));
    } catch {
      onToast(t("cart.copy.fail"));
    }
  };
  return (
    <>
      <h2 className="admin__h2">{t("admin.keys")}</h2>
      <p className="admin__hint">{t("admin.keys.hint")} <a href="https://github.com/Racion-App/racion/blob/main/docs/api.md" target="_blank" rel="noopener">docs/api.md</a></p>
      {fresh && (
        <div className="keybox" role="status">
          <b>{fresh.name}</b>
          <code className="keybox__key">{fresh.key}</code>
          <button type="button" className="btn btn-primary btn-sm" onClick={() => copy(fresh.key ?? "")}>
            <Copy size={14} aria-hidden /> {t("admin.keys.copy")}
          </button>
          <small>{t("admin.keys.once")}</small>
        </div>
      )}
      {keys?.map((k) => (
        <div className="keyrow" key={k.id}>
          <KeyRound size={16} aria-hidden />
          <span className="keyrow__main">
            <b>{k.name}</b> <code>{k.prefix}</code>
            <small>{k.lastUsedAt ? t("admin.keys.used", { date: dateShort(k.lastUsedAt.slice(0, 10), lang) }) : t("admin.keys.unused")}</small>
          </span>
          <button
            type="button"
            className="planrow__del"
            aria-label={t("delete")}
            onClick={async () => {
              if (!(await confirm({ title: `${t("admin.keys.revoke")}: ${k.name}?`, ok: t("delete"), danger: true }))) return;
              await api.apiKeyDelete(k.id).catch((e: Error) => onToast(e.message));
              void load();
            }}
          >
            <Trash2 size={16} aria-hidden />
          </button>
        </div>
      ))}
      <form className="keyform" onSubmit={create}>
        <input className="form-control" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("admin.keys.name")} maxLength={40} />
        <button type="submit" className="btn btn-soft">
          <Plus size={16} aria-hidden /> {t("admin.keys.create")}
        </button>
      </form>
    </>
  );
}
