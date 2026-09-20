import { useEffect, useState } from "react";
import { RefreshCw } from "lucide-react";
import { applyUpdate, startUpdateCheck } from "../lib/update";
import { useT } from "../i18n";

// Полоса внизу экрана: на сервере новая сборка, одна кнопка «Обновить». Появляется один раз за сессию.
export function UpdateBar() {
  const { t } = useT();
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false);
  useEffect(() => startUpdateCheck(() => setReady(true)), []);
  if (!ready) return null;
  return (
    <div className="update" role="status">
      <RefreshCw size={16} aria-hidden />
      <span>{t("update.ready")}</span>
      <button type="button" className="btn btn-primary btn-sm" disabled={busy} onClick={() => { setBusy(true); applyUpdate(); }}>{t("update.button")}</button>
    </div>
  );
}
