import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, ArrowRight, Check, ChefHat, Pause, Play, RotateCcw, TimerReset, X } from "lucide-react";
import type { Recipe } from "../lib/types";
import { qty } from "../lib/format";
import { useT } from "../i18n";

// Режим готовки: рецепт на весь экран, один шаг за раз, крупный текст, таймеры из текста шага
// («варить 20 минут» → кнопка), экран не гаснет (Wake Lock), продукты уже пересчитаны на порции.
// Таймеры живут в родителе и продолжают идти, пока листаешь шаги.

type Timer = { key: string; total: number; left: number; running: boolean; done: boolean };

// «20 минут», «15–20 мин», «1 час», «30 сек», «20 min», «2 Stunden», «10 分钟» → секунды. Из диапазона берём большее.
const UNITS: [RegExp, number][] = [
  [/^(сек|секунд[аыу]?|s|sec|seconds?|Sek(?:unden?)?|seg(?:undos?)?|sec(?:ondes?|ondi)?|sekund[ay]?|сек(?:унд[иа]?)?|saniye|秒)$/i, 1],
  [/^(мин|минут[аыу]?|min|mins?|minutes?|Min(?:uten?)?|minutos?|minuti|minut[y]?|хв|хвилин[аиу]?|dakika|dk|minuten|分钟?|分)$/i, 60],
  [/^(ч|час|часа|часов|h|hr|hrs|hours?|Std|Stunden?|horas?|heures?|or[ae]|godzin[yaę]?|год(?:ин[аиу]?)?|saat|сағат|uur|hodin[ya]?|小时|時間)$/i, 3600],
];
const DUR = /(\d+(?:[.,]\d+)?)(?:\s*[-–—]\s*(\d+(?:[.,]\d+)?))?\s*([^\s\d,.;:()]+)/g;

export function timersIn(text: string): number[] {
  const out: number[] = [];
  for (const m of text.matchAll(DUR)) {
    const unit = UNITS.find(([re]) => re.test(m[3]));
    if (!unit) continue;
    const n = parseFloat((m[2] ?? m[1]).replace(",", "."));
    const secs = Math.round(n * unit[1]);
    if (secs >= 10 && secs <= 24 * 3600 && !out.includes(secs)) out.push(secs);
  }
  return out;
}

function fmt(secs: number): string {
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = secs % 60;
  return (h ? `${h}:${String(m).padStart(2, "0")}` : String(m)) + ":" + String(s).padStart(2, "0");
}

function human(secs: number, min: string, hour: string): string {
  if (secs % 3600 === 0) return `${secs / 3600} ${hour}`;
  if (secs >= 60) return `${Math.round(secs / 60)} ${min}`;
  return `${secs} s`;
}

function beep() {
  try {
    const ac = new (window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext)();
    [0, 0.35, 0.7].forEach((at) => {
      const o = ac.createOscillator();
      const g = ac.createGain();
      o.type = "sine";
      o.frequency.value = 880;
      g.gain.setValueAtTime(0.0001, ac.currentTime + at);
      g.gain.exponentialRampToValueAtTime(0.3, ac.currentTime + at + 0.02);
      g.gain.exponentialRampToValueAtTime(0.0001, ac.currentTime + at + 0.25);
      o.connect(g).connect(ac.destination);
      o.start(ac.currentTime + at);
      o.stop(ac.currentTime + at + 0.3);
    });
  } catch {
    // без звука
  }
  try {
    navigator.vibrate?.([200, 100, 200, 100, 400]);
  } catch {
    // нет вибрации
  }
}

export function CookMode({ recipe, portions, onClose }: { recipe: Recipe; portions: number; onClose: () => void }) {
  const { t, lang } = useT();
  const ref = useRef<HTMLDialogElement>(null);
  const total = recipe.steps.length;
  const [step, setStep] = useState(0); // 0 — продукты, 1..total — шаги, total+1 — готово
  const [timers, setTimers] = useState<Record<string, Timer>>({});
  const stepTimers = useMemo(() => recipe.steps.map((s) => timersIn(s)), [recipe.steps]);

  useEffect(() => {
    const el = ref.current;
    if (el && !el.open) el.showModal();
    return () => {
      if (el?.open) el.close();
    };
  }, []);

  // экран не гаснет, пока открыт режим; после сворачивания вкладки блокировку надо запросить снова
  useEffect(() => {
    let lock: { release: () => Promise<void> } | null = null;
    const request = async () => {
      try {
        lock = await (navigator as Navigator & { wakeLock?: { request: (t: "screen") => Promise<{ release: () => Promise<void> }> } }).wakeLock?.request("screen") ?? null;
      } catch {
        lock = null;
      }
    };
    const onVisible = () => {
      if (document.visibilityState === "visible") void request();
    };
    void request();
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      document.removeEventListener("visibilitychange", onVisible);
      void lock?.release();
    };
  }, []);

  // тик всех запущенных таймеров
  useEffect(() => {
    const id = window.setInterval(() => {
      setTimers((prev) => {
        let changed = false;
        const next = { ...prev };
        for (const k of Object.keys(next)) {
          const tm = next[k];
          if (!tm.running) continue;
          changed = true;
          const left = tm.left - 1;
          if (left <= 0) {
            next[k] = { ...tm, left: 0, running: false, done: true };
            beep();
          } else next[k] = { ...tm, left };
        }
        return changed ? next : prev;
      });
    }, 1000);
    return () => window.clearInterval(id);
  }, []);

  const toggleTimer = (key: string, secs: number) => {
    setTimers((prev) => {
      const cur = prev[key] ?? { key, total: secs, left: secs, running: false, done: false };
      if (cur.done) return { ...prev, [key]: { ...cur, left: secs, running: true, done: false } };
      return { ...prev, [key]: { ...cur, running: !cur.running } };
    });
  };
  const resetTimer = (key: string) =>
    setTimers((prev) => {
      const { [key]: _gone, ...rest } = prev;
      return rest;
    });

  const go = useCallback((d: number) => setStep((s) => Math.max(0, Math.min(total + 1, s + d))), [total]);

  // стрелки на клавиатуре, свайп на телефоне, Esc — закрыть
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowRight") go(1);
      if (e.key === "ArrowLeft") go(-1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [go]);
  const touch = useRef<number | null>(null);
  const onTouchStart = (e: React.TouchEvent) => (touch.current = e.touches[0].clientX);
  const onTouchEnd = (e: React.TouchEvent) => {
    if (touch.current === null) return;
    const dx = e.changedTouches[0].clientX - touch.current;
    touch.current = null;
    if (Math.abs(dx) > 60) go(dx < 0 ? 1 : -1);
  };

  const running = Object.values(timers).filter((x) => x.running || x.done);
  const isKid = recipe.tags.includes("kidmenu");
  const p = isKid ? 1 : Math.max(1, Math.round(portions * 2) / 2);

  return (
    <dialog ref={ref} className="cook" aria-label={t("cook.title", { title: recipe.title })} onCancel={(e) => { e.preventDefault(); onClose(); }}>
      <header className="cook__head">
        <div className="cook__progress" role="progressbar" aria-valuemin={0} aria-valuemax={total} aria-valuenow={Math.min(step, total)}>
          {recipe.steps.map((_, i) => (
            <span key={i} className={i + 1 < step ? "is-done" : i + 1 === step ? "is-current" : ""} />
          ))}
        </div>
        <div className="cook__bar">
          <span className="cook__where">{step === 0 ? t("cook.products") : step > total ? t("cook.done.short") : t("cook.step", { n: step, total })}</span>
          <button type="button" className="theme-btn" onClick={onClose} aria-label={t("close")}>
            <X size={18} aria-hidden />
          </button>
        </div>
      </header>

      <div className="cook__body" onTouchStart={onTouchStart} onTouchEnd={onTouchEnd}>
        {step === 0 && (
          <section className="cook__card" key="ings">
            <h2 className="cook__title">{recipe.title}</h2>
            <p className="cook__sub">{t("cook.products.for", { n: p.toLocaleString() })}</p>
            <ul className="cook__ings">
              {recipe.ingredients.map((i) => (
                <li key={i.ingredientId} className={i.pantry ? "is-pantry" : ""}>
                  <span>{i.name}</span>
                  <span className="num">{qty(i.amount * p, i.unit, lang)}</span>
                </li>
              ))}
            </ul>
            <p className="cook__note">{t("cook.awake")}</p>
          </section>
        )}
        {step >= 1 && step <= total && (
          <section className="cook__card" key={"s" + step} aria-live="polite">
            <p className="cook__num num">{step}</p>
            <p className="cook__text">{recipe.steps[step - 1]}</p>
            {stepTimers[step - 1].length > 0 && (
              <div className="cook__timers">
                {stepTimers[step - 1].map((secs, i) => {
                  const key = `${step}-${i}`;
                  const tm = timers[key];
                  return (
                    <div key={key} className={"cook__timer" + (tm?.done ? " is-done" : tm?.running ? " is-running" : "")}>
                      <button type="button" className="btn btn-primary" onClick={() => toggleTimer(key, secs)}>
                        {tm?.done ? <RotateCcw size={18} aria-hidden /> : tm?.running ? <Pause size={18} aria-hidden /> : <Play size={18} aria-hidden />}
                        <span className="num">{tm ? fmt(tm.left) : t("cook.timer", { n: human(secs, t("min"), t("hour")) })}</span>
                      </button>
                      {tm && !tm.done && (
                        <button type="button" className="btn btn-soft" onClick={() => resetTimer(key)} aria-label={t("cook.reset")}>
                          <TimerReset size={18} aria-hidden />
                        </button>
                      )}
                      {tm?.done && <span className="cook__timer-done">{t("cook.timer.done")}</span>}
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        )}
        {step > total && (
          <section className="cook__card cook__card--done" key="done">
            <span className="cook__done-icon" aria-hidden>
              <ChefHat size={40} />
            </span>
            <h2 className="cook__title">{t("cook.done")}</h2>
            <p className="cook__sub">{recipe.title}</p>
            <button type="button" className="btn btn-primary btn-lg" onClick={onClose}>
              <Check size={18} aria-hidden /> {t("cook.finish")}
            </button>
          </section>
        )}
      </div>

      {running.length > 0 && step >= 1 && step <= total && (
        <div className="cook__running" aria-live="polite">
          {running.map((tm) => (
            <span key={tm.key} className={tm.done ? "is-done" : ""}>
              {t("cook.step.short", { n: tm.key.split("-")[0] })} · <span className="num">{fmt(tm.left)}</span>
            </span>
          ))}
        </div>
      )}

      {step <= total && (
        <footer className="cook__nav">
          <button type="button" className="btn btn-soft btn-lg" onClick={() => go(-1)} disabled={step === 0}>
            <ArrowLeft size={18} aria-hidden /> {t("cook.prev")}
          </button>
          <button type="button" className="btn btn-primary btn-lg" onClick={() => go(1)}>
            {step === 0 ? t("cook.start") : step === total ? t("cook.last") : t("cook.next")} <ArrowRight size={18} aria-hidden />
          </button>
        </footer>
      )}
    </dialog>
  );
}
