// Язык интерфейса. Словари живут на сервере в backend/locales/<code>.json и подгружаются по /api/locales/<code>;
// список языков — /api/locales. Выбор: localStorage (racion.lang) → cookie racion_lang → языки браузера →
// страна по IP (meta.geoLang, применяется приложением один раз) → ru. Cookie читает бэкенд.
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

export type Lang = string;

export type LocaleMeta = {
  code: string;
  name: string;
  english: string;
  flag: string;
  country: string;
  plural: "one-other" | "east-slavic" | "polish" | "czech" | "none" | string;
  decimal: string;
  months?: string[];
  monthsGen?: string[];
  date?: string;
  keys: number;
  hash: string; // версия файла: по ней кэшируется словарь и строится адрес запроса
};

type Dict = Record<string, string>;
const KEY = "racion.lang";
const CACHE = "racion.dict."; // + code → { v, dict }
const FALLBACK: Lang = "ru";

// Что известно синхронно до первого запроса: список языков приходит из /api/locales и кэшируется.
let LOCALES: LocaleMeta[] = readCache<LocaleMeta[]>("racion.locales") ?? [];
const DICTS: Record<string, Dict> = {};
let CURRENT: Lang = FALLBACK;

function readCache<T>(k: string): T | null {
  try {
    const v = localStorage.getItem(k);
    return v ? (JSON.parse(v) as T) : null;
  } catch {
    return null;
  }
}
function writeCache(k: string, v: unknown) {
  try {
    localStorage.setItem(k, JSON.stringify(v));
  } catch {
    // приватный режим
  }
}

export function isLang(v: unknown): v is Lang {
  return typeof v === "string" && /^[a-z]{2}$/.test(v) && (LOCALES.length === 0 ? ["ru", "en", "de"].includes(v) : LOCALES.some((l) => l.code === v));
}

export function localeMeta(l: Lang): LocaleMeta | undefined {
  return LOCALES.find((x) => x.code === l);
}
export function locales(): LocaleMeta[] {
  return LOCALES;
}

// Первый подходящий язык из настроек браузера.
function fromNavigator(): Lang | null {
  for (const raw of navigator.languages ?? [navigator.language]) {
    const code = (raw || "").slice(0, 2).toLowerCase();
    if (isLang(code)) return code;
  }
  return null;
}

// Явный выбор человека (хранилище или cookie) — без него язык считается автоматическим.
export function storedLang(): Lang | null {
  const v = readCache<string>(KEY);
  if (isLang(v)) return v;
  const m = document.cookie.match(/(?:^|; )racion_lang=(\w+)/);
  if (m && isLang(m[1])) return m[1];
  return null;
}

export function getLang(): Lang {
  return storedLang() ?? fromNavigator() ?? FALLBACK;
}

export function persistLang(l: Lang) {
  writeCache(KEY, l);
  document.cookie = `racion_lang=${l};path=/;max-age=31536000;samesite=lax`;
  document.documentElement.lang = l;
}

// Страна по умолчанию для языка — из _meta локали; для ru/en/de известна и без сервера.
export function langCountry(l: Lang): string {
  return localeMeta(l)?.country ?? ({ ru: "RU", en: "US", de: "DE" } as Record<string, string>)[l] ?? "US";
}
// Локаль Intl для чисел и дат: код языка + страна.
export function intlLocale(l: Lang): string {
  return `${l}-${langCountry(l)}`;
}
export function decimalSep(l: Lang): string {
  return localeMeta(l)?.decimal ?? (l === "en" ? "." : ",");
}

// Сервер может быть недоступен секунды (перезапуск, обрыв): тогда берём кэш, а не показываем сырые ключи.
async function loadLocales(): Promise<LocaleMeta[]> {
  try {
    const res = await fetch("/api/locales", { cache: "no-cache" });
    if (!res.ok) throw new Error("locales");
    const list = (await res.json()) as LocaleMeta[];
    LOCALES = list;
    writeCache("racion.locales", list);
    return list;
  } catch (e) {
    const cached = readCache<LocaleMeta[]>("racion.locales");
    if (cached && cached.length) {
      LOCALES = cached;
      return cached;
    }
    throw e;
  }
}

async function fetchRetry(url: string, tries = 3): Promise<Response> {
  let last: unknown;
  for (let i = 0; i < tries; i++) {
    try {
      const res = await fetch(url);
      if (res.ok) return res;
      last = new Error(String(res.status));
    } catch (e) {
      last = e;
    }
    await new Promise((r) => setTimeout(r, 1500 * (i + 1)));
  }
  throw last;
}

async function loadDict(l: Lang): Promise<Dict> {
  if (DICTS[l]) return DICTS[l];
  const meta = localeMeta(l);
  const cached = readCache<{ v: string | number; dict: Dict }>(CACHE + l);
  if (cached && meta && cached.v === meta.hash) {
    DICTS[l] = cached.dict;
    return cached.dict;
  }
  let raw: Record<string, unknown>;
  try {
    const res = await fetchRetry(`/api/locales/${l}?v=${meta?.hash ?? ""}`);
    raw = (await res.json()) as Record<string, unknown>;
  } catch (e) {
    if (cached) {
      DICTS[l] = cached.dict; // старый словарь лучше сырых ключей
      return cached.dict;
    }
    throw e;
  }
  const dict: Dict = {};
  for (const [k, v] of Object.entries(raw)) if (k !== "_meta" && typeof v === "string") dict[k] = v;
  DICTS[l] = dict;
  // при упреждающей загрузке список языков мог прийти позже запроса — хэш берём свежий
  writeCache(CACHE + l, { v: localeMeta(l)?.hash ?? meta?.hash ?? "", dict });
  return dict;
}

function format(s: string, params?: Record<string, string | number>): string {
  if (!params) return s;
  return s.replace(/\{(\w+)\}/g, (_, k: string) => (k in params ? String(params[k]) : `{${k}}`));
}

// Форма слова по числу по правилу языка из _meta: one | few | many.
export function pluralKey(lang: Lang, n: number): "one" | "few" | "many" {
  const rule = localeMeta(lang)?.plural ?? (lang === "ru" ? "east-slavic" : "one-other");
  const m10 = n % 10;
  const m100 = n % 100;
  switch (rule) {
    case "east-slavic":
      if (m10 === 1 && m100 !== 11) return "one";
      if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return "few";
      return "many";
    case "polish":
      if (n === 1) return "one";
      if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return "few";
      return "many";
    case "czech":
      if (n === 1) return "one";
      if (n >= 2 && n <= 4) return "few";
      return "many";
    case "none":
      return "many";
    default:
      return n === 1 ? "one" : "many";
  }
}

// Чего нет в языке — берём из английского, потом из русского (оба словаря подгружаются запасом).
function lookup(lang: Lang, key: string): string {
  return DICTS[lang]?.[key] ?? DICTS.en?.[key] ?? DICTS[FALLBACK]?.[key] ?? key;
}

type Ctx = { lang: Lang; setLang: (l: Lang) => void; ready: boolean; auto: boolean };
const LangCtx = createContext<Ctx>({ lang: FALLBACK, setLang: () => {}, ready: false, auto: true });

export function LangProvider({ children }: { children: ReactNode }) {
  const [lang, set] = useState<Lang>(() => {
    const l = getLang();
    document.documentElement.lang = l;
    CURRENT = l;
    return l;
  });
  const [ready, setReady] = useState(false);
  const auto = storedLang() === null;

  // список языков, словарь текущего и русский как запас
  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        // первый визит: словарь текущего языка запрашиваем сразу, параллельно со списком языков —
        // это убирает один круг до первой отрисовки; при смене языка ниже он просто не пригодится
        const early = !DICTS[lang] && !readCache<unknown>(CACHE + lang) ? loadDict(lang).catch(() => ({})) : null;
        await loadLocales();
        // до загрузки списка языков браузерный язык мог не распознаться (список ещё не в кэше) — проверяем ещё раз
        if (auto) {
          const nav = fromNavigator();
          if (nav && nav !== lang) {
            CURRENT = nav;
            document.documentElement.lang = nav;
            set(nav);
            return;
          }
        }
        // язык не выбран и браузер ничего подходящего не сказал — спросим страну по IP (один раз за сессию)
        if (auto && !fromNavigator() && !sessionStorage.getItem("racion.lang.geo")) {
          sessionStorage.setItem("racion.lang.geo", "1");
          const hint = (await (await fetch("/api/lang")).json()) as { geo?: string };
          if (hint.geo && isLang(hint.geo) && hint.geo !== lang) {
            CURRENT = hint.geo;
            document.documentElement.lang = hint.geo;
            set(hint.geo);
            return;
          }
        }
        await Promise.all([early ?? loadDict(lang), lang !== FALLBACK ? loadDict(FALLBACK) : Promise.resolve({}), lang !== "en" && lang !== FALLBACK ? loadDict("en") : Promise.resolve({})]);
      } catch {
        // без сети покажем ключи; страница всё равно работает
      }
      if (alive) setReady(true);
    })();
    return () => {
      alive = false;
    };
  }, [lang]);

  const setLang = useCallback((l: Lang) => {
    if (!isLang(l)) return;
    persistLang(l);
    CURRENT = l;
    setReady(false);
    set(l);
  }, []);
  const value = useMemo(() => ({ lang, setLang, ready, auto }), [lang, setLang, ready, auto]);
  if (!ready && !DICTS[lang]) return null; // словарь грузится с того же сервера, это десятки миллисекунд
  return <LangCtx.Provider value={value}>{children}</LangCtx.Provider>;
}

export function useLang() {
  return useContext(LangCtx);
}

export function useT() {
  const { lang, ready } = useLang();
  return useMemo(() => {
    const t = (key: string, params?: Record<string, string | number>) => format(lookup(lang, key), params);
    // tn("items", 5) → «позиций»; ключи key.one / key.few / key.many
    const tn = (key: string, n: number) => {
      const form = pluralKey(lang, n);
      return DICTS[lang]?.[`${key}.${form}`] !== undefined || DICTS.en?.[`${key}.${form}`] !== undefined || DICTS[FALLBACK]?.[`${key}.${form}`] !== undefined ? t(`${key}.${form}`) : t(`${key}.many`);
    };
    return { t, tn, lang };
  }, [lang, ready]);
}

// Прямой доступ вне React (ошибки api и т.п.).
export function tStatic(key: string, params?: Record<string, string | number>): string {
  return format(lookup(CURRENT, key), params);
}
