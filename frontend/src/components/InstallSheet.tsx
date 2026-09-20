import { useEffect, useState } from "react";
import { Bell, Download, Smartphone, WifiOff, Check } from "lucide-react";
import { InstallMock, type MockKind } from "./InstallMock";
import { Sheet } from "./Sheet";
import { canPrompt, dismissSuggest, installPlatform, iosMajor, isStandalone, onInstallChange, promptInstall } from "../lib/install";
import { track } from "../lib/analytics";
import { useT } from "../i18n";

// Лист «Установить Рацион»: три пользы, кнопка установки там, где браузер умеет ставить сам (Android, Chrome,
// Edge), и пошаговая инструкция с макетами экранов (InstallMock) для iPhone, Android и ПК.

type Step = { text: string; mock: MockKind };

export function InstallSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useT();
  const [prompt, setPrompt] = useState(canPrompt());
  const [done, setDone] = useState(isStandalone());
  useEffect(() => onInstallChange(() => { setPrompt(canPrompt()); setDone(isStandalone()); }), []);
  const platform = installPlatform();
  const steps: Step[] =
    platform === "ios"
      ? [
          // iOS 26: «Поделиться» внутри меню «…» справа внизу; раньше — отдельная кнопка в панели Safari
          iosMajor() >= 26 ? { text: t("install.ios26.1"), mock: "menu26" } : { text: t("install.ios.1"), mock: "share18" },
          { text: t("install.ios.2"), mock: "addhome" },
          { text: t("install.ios.3"), mock: "add" },
        ]
      : platform === "android"
        ? [{ text: t("install.android.1"), mock: "amenu" }, { text: t("install.android.2"), mock: "ainstall" }, { text: t("install.android.3"), mock: "aconfirm" }]
        : [{ text: t("install.desktop.1"), mock: "omnibox" }, { text: t("install.desktop.2"), mock: "dconfirm" }];
  const install = async () => {
    track("pwa_prompt");
    const r = await promptInstall();
    if (r === "accepted") { track("pwa_installed"); setDone(true); }
  };
  const close = () => { dismissSuggest(); onClose(); };
  return (
    <Sheet open={open} onClose={close} title={t("install.title")} closeLabel={t("close")}>
      <h2>{t("install.title")}</h2>
      <p className="bsheet__lead">{t("install.lead")}</p>
      <ul className="install__benefits">
        <li><WifiOff size={18} aria-hidden /> {t("install.b1")}</li>
        <li><Bell size={18} aria-hidden /> {t("install.b2")}</li>
        <li><Smartphone size={18} aria-hidden /> {t("install.b3")}</li>
      </ul>
      {done ? (
        <p className="install__done"><Check size={16} aria-hidden /> {t("install.installed")}</p>
      ) : prompt ? (
        <div className="install__actions">
          <button type="button" className="btn btn-primary btn-lg" onClick={install}><Download size={18} aria-hidden /> {t("install.button")}</button>
          <button type="button" className="btn btn-soft btn-lg" onClick={close}>{t("install.later")}</button>
        </div>
      ) : (
        <>
          <h3>{t("install.how")}</h3>
          <ol className="install__steps">
            {steps.map((s, i) => (
              <li key={i}><InstallMock kind={s.mock} /><span className="install__text"><b className="num">{i + 1}</b>{s.text}</span></li>
            ))}
          </ol>
          {platform !== "ios" && <p className="quiz__hint">{t("install.other")}</p>}
          <div className="install__actions"><button type="button" className="btn btn-soft btn-lg" onClick={close}>{t("install.later")}</button></div>
        </>
      )}
    </Sheet>
  );
}

// Подсказка на телефоне: тонкая полоса под шапкой, раз в две недели, только пока приложение не установлено
export function InstallNudge() {
  const { t } = useT();
  const [show, setShow] = useState(false);
  const [open, setOpen] = useState(false);
  useEffect(() => {
    // предлагаем не с порога: после того как человек что-то сделал (открыл план) или на втором заходе
    let visits = 0;
    try {
      visits = Number(localStorage.getItem("racion.visits") || 0) + 1;
      localStorage.setItem("racion.visits", String(visits));
    } catch {
      // приватный режим
    }
    import("../lib/install").then((m) => setShow(m.shouldSuggest() && (visits >= 2 || location.pathname.startsWith("/plan/"))));
  }, []);
  if (!show) return null;
  return (
    <>
      <button type="button" className="install__nudge" onClick={() => { setOpen(true); track("pwa_nudge_open"); }}>
        <Smartphone size={16} aria-hidden />
        <span><b>{t("install.card")}</b><small>{t("install.card.sub")}</small></span>
        <Download size={16} aria-hidden />
      </button>
      <InstallSheet open={open} onClose={() => { setOpen(false); setShow(false); }} />
    </>
  );
}

// Карточка в кабинете: установлено или нет, кнопка открывает лист
export function InstallCard() {
  const { t } = useT();
  const [open, setOpen] = useState(false);
  const [done, setDone] = useState(isStandalone());
  useEffect(() => onInstallChange(() => setDone(isStandalone())), []);
  return (
    <div className="account__card install__card">
      <span className="install__card-icon"><Smartphone size={18} aria-hidden /></span>
      <span className="install__card-text"><b>{t("install.card")}</b><small>{done ? t("install.installed") : t("install.card.sub")}</small></span>
      {done ? <Check size={18} className="install__card-done" aria-hidden /> : <button type="button" className="btn btn-soft btn-sm" onClick={() => setOpen(true)}>{t("install.button")}</button>}
      <InstallSheet open={open} onClose={() => setOpen(false)} />
    </div>
  );
}
