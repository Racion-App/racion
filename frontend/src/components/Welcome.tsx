import { useCallback, useState } from "react";
import { ListChecks, ReceiptText, Users, Utensils } from "lucide-react";
import { Sheet } from "./Sheet";
import { track } from "../lib/analytics";
import { useT } from "../i18n";

// Первый визит: один лист «что умеет Рацион» с четырьмя пунктами и кнопкой «Начать».
// Показывается один раз (localStorage racion.welcomed), «Пропустить» — то же самое без отметки в аналитике.

const KEY = "racion.welcomed";

function seen(): boolean {
  try {
    return localStorage.getItem(KEY) === "1";
  } catch {
    return true; // без хранилища не показываем — иначе будет всплывать каждый раз
  }
}
function markSeen() {
  try {
    localStorage.setItem(KEY, "1");
  } catch {
    // приватный режим
  }
}

const POINTS = [
  { key: "menu", Icon: ReceiptText },
  { key: "list", Icon: ListChecks },
  { key: "family", Icon: Users },
  { key: "own", Icon: Utensils },
] as const;

export function Welcome() {
  const { t } = useT();
  const [open, setOpen] = useState(() => !seen());
  const close = useCallback((how: "start" | "skip") => {
    markSeen();
    setOpen(false);
    track("welcome_" + how);
  }, []);
  if (!open) return null;
  return (
    <Sheet open={open} onClose={() => close("skip")} title={t("welcome.title")} closeLabel={t("close")}>
      <div className="welcome">
        <h2 className="welcome__title">{t("welcome.title")}</h2>
        <p className="bsheet__lead">{t("welcome.lead")}</p>
        <ul className="welcome__list">
          {POINTS.map(({ key, Icon }) => (
            <li key={key} className="welcome__item">
              <span className="welcome__icon" aria-hidden>
                <Icon size={22} />
              </span>
              <span>
                <b>{t(`welcome.${key}`)}</b>
                <small>{t(`welcome.${key}.text`)}</small>
              </span>
            </li>
          ))}
        </ul>
        <div className="welcome__actions">
          <button type="button" className="btn btn-primary btn-lg" onClick={() => close("start")} autoFocus>
            {t("welcome.start")}
          </button>
          <button type="button" className="btn btn-link" onClick={() => close("skip")}>
            {t("welcome.skip")}
          </button>
        </div>
      </div>
    </Sheet>
  );
}
