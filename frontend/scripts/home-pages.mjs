// dist/<lang>/index.html — та же оболочка приложения, но с head своего языка: title, description,
// canonical, hreflang и текст для роботов без JS. Без этого /en и /de индексируются как копии русской главной.
// Тексты: home-seo.json (генерируется scripts/seo/gen_home_seo.py из локалей и подборок).
//
// Главные (/, /en, /de…) ещё и показывают первый шаг квиза прямо в HTML, а не крутилку: на телефоне
// приложение запускается за две-три секунды, и всё это время человек видел пустой экран. Тексты шага —
// из словарей бэкенда (home-shell.json; в докере бэкенда рядом нет, поэтому файл в репозитории и
// обновляется при каждой локальной сборке). React потом рисует тот же шаг поверх.
// dist/index.html остаётся без первого шага: это оболочка и для недели, и для кабинета.
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";

const dist = path.resolve("dist");
const seo = JSON.parse(fs.readFileSync(path.resolve("home-seo.json"), "utf8"));
const src = fs.readFileSync(path.join(dist, "index.html"), "utf8");
const base = "https://racion.app";
const langs = Object.keys(seo);
const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

// ── Тексты первого шага ────────────────────────────────────────────────────

const SHELL_KEYS = ["brand", "brand.home", "nav.recipes", "quiz.promise", "quiz.sample", "dishes.one", "dishes.many", "items.many", "quiz.trust", "quiz.q1", "quiz.q1.hint", "quiz.q1.hint.RU", "quiz.country"];
const shellFile = path.resolve("home-shell.json");
const localesDir = path.resolve("../backend/locales");

function shellTexts() {
  if (!fs.existsSync(localesDir)) {
    const saved = JSON.parse(fs.readFileSync(shellFile, "utf8"));
    for (const lang of langs) {
      const miss = SHELL_KEYS.filter((k) => typeof saved[lang]?.[k] !== "string");
      if (miss.length) throw new Error(`home-shell.json устарел (${lang}: ${miss.join(", ")}) — соберите фронт локально рядом с backend/locales`);
    }
    return saved;
  }
  const read = (l) => JSON.parse(fs.readFileSync(path.join(localesDir, `${l}.json`), "utf8"));
  const ru = read("ru");
  const en = read("en");
  const out = {};
  for (const lang of langs) {
    const d = read(lang);
    const t = {};
    for (const k of SHELL_KEYS) {
      const v = d[k] ?? en[k] ?? ru[k]; // как в приложении: язык → английский → русский
      if (typeof v !== "string" || (v.includes("{") && k !== "quiz.sample")) throw new Error(`home-shell: ${lang} ${k}`);
      t[k] = v;
    }
    t.country = d._meta?.country ?? "US"; // страна по умолчанию для языка — с ней приложение просит справочник
    t.plural = d._meta?.plural ?? "one-other";
    out[lang] = t;
  }
  fs.writeFileSync(shellFile, JSON.stringify(out, null, 1) + "\n");
  return out;
}

const texts = shellTexts();

const RECEIPT = '<svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-receipt-text" aria-hidden="true"><path d="M4 2v20l2-1 2 1 2-1 2 1 2-1 2 1 2-1 2 1V2l-2 1-2-1-2 1-2-1-2 1-2-1-2 1Z"></path><path d="M14 8H8"></path><path d="M16 12H8"></path><path d="M13 16H8"></path></svg>';

// первый шаг квиза теми же классами, что рисует Quiz.tsx; там, где нужен справочник, — те же заглушки
function shell(lang) {
  const t = texts[lang];
  const p = lang === "ru" ? "" : `/${lang}`;
  const hint = t.country === "RU" ? t["quiz.q1.hint.RU"] : t["quiz.q1.hint"];
  const progress = Array.from({ length: 7 }, (_, i) => (i === 0 ? '<span class="is-current"></span>' : "<span></span>")).join("");
  const tiles = Array.from({ length: 9 }, () => '<div class="skeleton" style="min-height:88px"></div>').join("");
  // строка с примером недели приходит со справочником; до него — невидимая заготовка той же длины, как в Quiz.tsx
  const dishes = t.plural === "east-slavic" ? t["dishes.one"] : t["dishes.many"]; // 21 по pluralKey: «21 блюдо», «21 dishes»
  const sample = `<p class="quiz__sample" aria-hidden="true" style="visibility:hidden">${esc(t["quiz.sample"].replace("{store}", "Пятёрочка"))} <span class="quiz__fact"><span class="num">21</span> ${esc(dishes)}</span> · <span class="quiz__fact num">≈ 6 888 ₽</span> · <span class="quiz__fact"><span class="num">65</span> ${esc(t["items.many"])}</span></p>`;
  return `<div class="shell" id="homeshell">
        <header class="topbar"><a href="${p || "/"}" class="topbar__brand" aria-label="${esc(t["brand.home"])}">${RECEIPT}${esc(t.brand)}</a><div class="topbar__right pages-nav"><a href="${p}/recipes" class="pages-nav__link">${esc(t["nav.recipes"])}</a><span class="theme-btn"></span><span class="theme-btn"></span><span class="theme-btn"></span></div></header>
        <main class="quiz"><div class="quiz__progress">${progress}</div><section class="quiz__step"><div class="quiz__promise"><p class="quiz__lead">${esc(t["quiz.promise"])}</p>${sample}<p class="quiz__trust">${esc(t["quiz.trust"])}</p></div><h1 class="quiz__title">${esc(t["quiz.q1"])}</h1><p class="quiz__hint" elementtiming="homeshell">${esc(hint)}</p><div class="quiz__group quiz__group--tight"><span class="quiz__label">${esc(t["quiz.country"])}</span><div class="skeleton" style="min-height:44px"></div></div><div class="tiles">${tiles}</div></section></main>
      </div>
      <script>${GUARD}</script>`;
}

// Страж: первый шаг остаётся, только если React нарисует то же самое. Иначе его место займёт крутилка:
// другой язык (выбран раньше или у телефона), анкета не с первого шага, событие или подборка в адресе,
// не главная (service worker отдаёт эту же страницу и для недели, когда нет сети).
// Скрипт один на все языки — его хеш стоит в CSP (frontend/nginx.conf), сборка это проверяет.
const GUARD = '(function(){try{var d=document,s=d.getElementById("homeshell");if(!s)return;var L="ru en de es fr it pt pl uk tr kk nl cs zh ja".split(" "),q=new URLSearchParams(location.search),g=location.pathname.split("/"),pre=L.indexOf(g[1])>=0,home=location.pathname==="/"||(pre&&g.length<=3&&!g[2]),l=pre?g[1]:(q.get("lang")||(localStorage.getItem("racion.lang")||"").replace(/"/g,"")||(d.cookie.match(/(?:^|; )racion_lang=(\\w+)/)||[])[1]);if(!l){var n=navigator.languages||[navigator.language];for(var i=0;i<n.length&&!l;i++){var c=(n[i]||"").slice(0,2).toLowerCase();if(L.indexOf(c)>=0)l=c}}if(!home||(l&&l!==d.documentElement.lang)||Number(q.get("s"))>1||q.has("event")||q.has("collection"))s.remove()}catch(e){}})()';

const guardHash = "sha256-" + crypto.createHash("sha256").update(GUARD, "utf8").digest("base64");
if (!fs.readFileSync(path.resolve("nginx.conf"), "utf8").includes(`'${guardHash}'`)) {
  throw new Error(`home-pages: хеша стража нет в CSP nginx.conf, добавьте '${guardHash}' в script-src`);
}

// ── Страницы ───────────────────────────────────────────────────────────────

const alternates = (pathFor) =>
  langs.map((l) => `<link rel="alternate" hreflang="${l}" href="${base}${pathFor(l)}" />`).join("\n    ") +
  `\n    <link rel="alternate" hreflang="x-default" href="${base}/" />`;

function render(lang, withShell) {
  const { title, description, links } = seo[lang];
  const self = lang === "ru" ? "/" : `/${lang}`;
  let html = src;
  html = html.replace(/<html lang="[^"]*"/, `<html lang="${lang}"`);
  html = html.replace(/<title>[^<]*<\/title>/, `<title>${esc(title)}</title>`);
  html = html.replace(/(<meta name="description" content=")[^"]*(")/, `$1${esc(description)}$2`);
  html = html.replace(/(<meta property="og:title" content=")[^"]*(")/, `$1${esc(title)}$2`);
  html = html.replace(/(<meta property="og:description" content=")[^"]*(")/, `$1${esc(description)}$2`);
  html = html.replace(/(<meta property="og:locale" content=")[^"]*(")/, `$1${lang}$2`);
  html = html.replace(/(<meta property="og:image" content="\/og\/)[a-z]{2}(\.png")/, `$1${lang}$2`);
  html = html.replace(/"inLanguage":"[a-z]{2}"/, `"inLanguage":"${lang}"`);
  html = html.replace(/<link rel="canonical"[^>]*>/, `<link rel="canonical" href="${base}${self}" />\n    ${alternates((l) => (l === "ru" ? "/" : `/${l}`))}`);
  // предзагрузка того, что приложение попросит первым: словарь и справочник этого языка, а не русские
  html = html.replace(/<link rel="preload" href="\/api\/meta"[^>]*>/, `<link rel="preload" href="/api/meta?country=${texts[lang].country}&amp;lang=${lang}" as="fetch" crossorigin />`);
  html = html.replace(/<link rel="preload" href="\/api\/locales\/[a-z]{2}\?v="/, `<link rel="preload" href="/api/locales/${lang}?v="`);
  // текст для роботов без JS: заголовок, лид и ссылки на серверные страницы того же языка
  const list = links.map((l) => `          <li><a href="${l.href}">${esc(l.text)}</a></li>`).join("\n");
  html = html.replace(/<main class="prerender">[\s\S]*?<\/main>/,
    `<main class="prerender">\n        <h1>${esc(title.split(" — ")[0])}</h1>\n        <p>${esc(description)}</p>\n        <ul>\n${list}\n        </ul>\n      </main>`);
  if (withShell) {
    const n = html.length;
    html = html.replace(/(<div id="root">\s*)/, `$1${shell(lang)}\n      `);
    if (html.length === n) throw new Error("home-pages: нет #root");
  }
  return html;
}

for (const lang of langs) {
  if (lang === "ru") {
    fs.writeFileSync(path.join(dist, "index.html"), render(lang, false)); // оболочка всех страниц приложения
    fs.writeFileSync(path.join(dist, "home.html"), render(lang, true)); // главная: nginx отдаёт её на «/»
    continue;
  }
  fs.mkdirSync(path.join(dist, lang), { recursive: true });
  fs.writeFileSync(path.join(dist, lang, "index.html"), render(lang, true));
}
console.log("home pages:", langs.join(" "), "| страж", guardHash);
