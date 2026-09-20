import { useEffect, useState } from "react";
import { Bell, Download, Smartphone, WifiOff, Check } from "lucide-react";
import { Sheet } from "./Sheet";
import { canPrompt, dismissSuggest, installPlatform, iosMajor, isStandalone, onInstallChange, promptInstall } from "../lib/install";
import { track } from "../lib/analytics";
import { useT } from "../i18n";

// Лист «Установить Рацион»: три пользы, кнопка установки там, где браузер умеет ставить сам (Android, Chrome,
// Edge), и пошаговая инструкция для iPhone и прочих. Для iPhone — вырезки из настоящих экранов Safari с кольцом
// на нужной кнопке (public/images/install), для остальных — схемы экрана. Если фото не загрузилось, схема.

type Art = "share" | "addhome" | "add" | "menu" | "installmenu" | "omnibox" | "confirm";
// shot: файл вырезки 4:5 и зоны-акценты в процентах от её ширины и высоты: [x, y, w, h] — рамка на строке
// или кнопке; порядок зон — порядок нажатий
type Zone = [number, number, number, number];
type Step = { text: string; art: Art; shot?: { src: string; zones: Zone[] } };

// Вырезка экрана с рамками на кнопках; при ошибке загрузки — схема
function Shot({ step }: { step: Step }) {
  const [broken, setBroken] = useState(false);
  if (!step.shot || broken) return <Art kind={step.art} />;
  return (
    <span className="install__shot" aria-hidden>
      <img src={step.shot.src} alt="" width="480" height="600" loading="lazy" decoding="async" onError={() => setBroken(true)} />
      {step.shot.zones.map(([x, y, w, h], i) => <i key={i} style={{ left: `${x}%`, top: `${y}%`, width: `${w}%`, height: `${h}%` }} />)}
    </span>
  );
}

// Схема телефона с одним акцентом: где нажимать
function Art({ kind }: { kind: Step["art"] }) {
  const stroke = "var(--ds-label-tertiary)";
  const accent = "var(--ds-blue)";
  return (
    <svg className="install__art" viewBox="0 0 120 160" width="90" height="120" aria-hidden>
      <rect x="6" y="4" width="108" height="152" rx="16" fill="var(--ds-bg-elevated)" stroke={stroke} strokeWidth="2" />
      <rect x="14" y="16" width="92" height="128" rx="8" fill="var(--ds-fill-4)" />
      {kind === "share" && (
        <>
          <rect x="14" y="128" width="92" height="16" fill="var(--ds-bg-elevated)" />
          <path d="M60 126v-10m0 0-4 4m4-4 4 4M52 122h-2a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h20a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-2" fill="none" stroke={accent} strokeWidth="2" strokeLinecap="round" />
          <circle cx="60" cy="128" r="14" fill="none" stroke={accent} strokeWidth="2" strokeDasharray="3 3" />
        </>
      )}
      {kind === "addhome" && (
        <>
          <rect x="14" y="60" width="92" height="84" rx="8" fill="var(--ds-bg-elevated)" />
          {[70, 84, 98].map((y) => <rect key={y} x="22" y={y} width="76" height="8" rx="3" fill="var(--ds-fill-3)" />)}
          <rect x="22" y="112" width="76" height="10" rx="3" fill="var(--ds-blue-tint)" stroke={accent} strokeWidth="1.5" />
          <rect x="26" y="114" width="6" height="6" rx="1.5" fill="none" stroke={accent} strokeWidth="1.5" />
          <path d="M29 115.5v3M27.5 117h3" stroke={accent} strokeWidth="1.2" strokeLinecap="round" />
        </>
      )}
      {kind === "add" && (
        <>
          <rect x="14" y="16" width="92" height="20" fill="var(--ds-bg-elevated)" />
          <rect x="80" y="21" width="20" height="10" rx="3" fill={accent} />
          <rect x="30" y="52" width="24" height="24" rx="6" fill={accent} />
          <path d="M36 58h12M36 64h12M36 70h8" stroke="#fff" strokeWidth="2" strokeLinecap="round" />
          <rect x="30" y="80" width="24" height="5" rx="2" fill="var(--ds-fill-2)" />
        </>
      )}
      {kind === "menu" && (
        <>
          <rect x="14" y="16" width="92" height="18" fill="var(--ds-bg-elevated)" />
          <circle cx="94" cy="22" r="1.8" fill={accent} /><circle cx="94" cy="27" r="1.8" fill={accent} /><circle cx="94" cy="32" r="1.8" fill={accent} />
          <circle cx="94" cy="27" r="9" fill="none" stroke={accent} strokeWidth="2" strokeDasharray="3 3" />
        </>
      )}
      {kind === "installmenu" && (
        <>
          <rect x="40" y="20" width="62" height="80" rx="6" fill="var(--ds-bg-elevated)" stroke={stroke} strokeWidth="1" />
          {[30, 44, 72, 86].map((y) => <rect key={y} x="46" y={y} width="50" height="6" rx="2" fill="var(--ds-fill-3)" />)}
          <rect x="44" y="55" width="54" height="12" rx="3" fill="var(--ds-blue-tint)" stroke={accent} strokeWidth="1.5" />
          <path d="M50 58v6m-2.5-2.5 2.5 2.5 2.5-2.5" stroke={accent} strokeWidth="1.3" strokeLinecap="round" fill="none" />
        </>
      )}
      {kind === "omnibox" && (
        <>
          <rect x="14" y="16" width="92" height="18" fill="var(--ds-bg-elevated)" />
          <rect x="20" y="20" width="70" height="10" rx="5" fill="var(--ds-fill-3)" />
          <path d="M96 21v6m-2.5-2.5 2.5 2.5 2.5-2.5" stroke={accent} strokeWidth="1.5" strokeLinecap="round" fill="none" />
          <circle cx="96" cy="25" r="7" fill="none" stroke={accent} strokeWidth="1.5" strokeDasharray="3 3" />
        </>
      )}
      {kind === "confirm" && (
        <>
          <rect x="24" y="56" width="72" height="48" rx="8" fill="var(--ds-bg-elevated)" stroke={stroke} strokeWidth="1" />
          <rect x="32" y="64" width="40" height="6" rx="2" fill="var(--ds-fill-2)" />
          <rect x="32" y="74" width="56" height="5" rx="2" fill="var(--ds-fill-3)" />
          <rect x="60" y="86" width="30" height="12" rx="4" fill={accent} />
          <path d="M70 92l3 3 6-6" stroke="#fff" strokeWidth="2" strokeLinecap="round" fill="none" />
        </>
      )}
    </svg>
  );
}

export function InstallSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useT();
  const [prompt, setPrompt] = useState(canPrompt());
  const [done, setDone] = useState(isStandalone());
  useEffect(() => onInstallChange(() => { setPrompt(canPrompt()); setDone(isStandalone()); }), []);
  const platform = installPlatform();
  const SHOTS = "/images/install/";
  const steps: Step[] =
    platform === "ios"
      ? [
          // iOS 26: «Поделиться» внутри меню «…» справа внизу; раньше — отдельная кнопка в панели Safari
          iosMajor() >= 26
            ? { text: t("install.ios26.1"), art: "share", shot: { src: SHOTS + "ios-menu.webp", zones: [[76, 80, 18, 14], [20, 7.6, 58, 8.2]] } }
            : { text: t("install.ios.1"), art: "share" },
          { text: t("install.ios.2"), art: "addhome", shot: { src: SHOTS + "ios-addhome.webp", zones: [[1.5, 76.6, 92, 9]] } },
          { text: t("install.ios.3"), art: "add", shot: { src: SHOTS + "ios-add.webp", zones: [[66, 5.5, 29, 9]] } },
        ]
      : platform === "android"
        ? [{ text: t("install.android.1"), art: "menu" }, { text: t("install.android.2"), art: "installmenu" }, { text: t("install.android.3"), art: "confirm" }]
        : [{ text: t("install.desktop.1"), art: "omnibox" }, { text: t("install.desktop.2"), art: "confirm" }];
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
              <li key={i}><Shot step={s} /><span><b className="num">{i + 1}</b>{s.text}</span></li>
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
