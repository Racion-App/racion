import { useRef, useState } from "react";
import { Camera, Trash2 } from "lucide-react";
import { api, ApiError } from "../lib/api";
import { useT } from "../i18n";

// Поле «фото»: превью, выбор файла с камеры или из галереи, загрузка на сервер (WebP в S3) и удаление.
// Наружу отдаёт ссылку из нашего хранилища; сервер её проверяет при сохранении.

export function PhotoField({ value, onChange, kind, round, label }: { value: string; onChange: (url: string, thumb: string) => void; kind: "recipe" | "comment" | "avatar" | "offer"; round?: boolean; label?: string }) {
  const { t } = useT();
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const pick = async (file: File | undefined) => {
    if (!file) return;
    setErr(null);
    setBusy(true);
    try {
      const p = await api.upload(kind, file);
      onChange(p.url, p.thumb);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : t("photo.bad"));
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  };

  return (
    <div className={"photo" + (round ? " photo--round" : "") + (busy ? " is-busy" : "")}>
      <button type="button" className="photo__pick" onClick={() => input.current?.click()} disabled={busy} aria-label={value ? t("photo.change") : label ?? t("photo.add")}>
        {value ? <img src={value} alt="" /> : <Camera size={round ? 20 : 24} aria-hidden />}
        {busy && <span className="photo__busy">{t("photo.uploading")}</span>}
      </button>
      <div className="photo__side">
        <button type="button" className="btn btn-soft btn-sm" onClick={() => input.current?.click()} disabled={busy}>
          <Camera size={14} aria-hidden /> {value ? t("photo.change") : label ?? t("photo.add")}
        </button>
        {value && (
          <button type="button" className="btn btn-link btn-sm" onClick={() => onChange("", "")} disabled={busy}>
            <Trash2 size={14} aria-hidden /> {t("photo.remove")}
          </button>
        )}
        <small>{err ?? t("photo.hint")}</small>
      </div>
      <input ref={input} type="file" accept="image/*" hidden onChange={(e) => pick(e.target.files?.[0])} />
    </div>
  );
}
