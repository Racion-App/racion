// dist/<lang>/index.html — та же оболочка приложения, но с head своего языка: title, description,
// canonical, hreflang и текст для роботов без JS. Без этого /en и /de индексируются как копии русской главной.
// Тексты: home-seo.json (генерируется scripts/seo/gen_home_seo.py из локалей и подборок).
import fs from "node:fs";
import path from "node:path";

const dist = path.resolve("dist");
const seo = JSON.parse(fs.readFileSync(path.resolve("home-seo.json"), "utf8"));
const src = fs.readFileSync(path.join(dist, "index.html"), "utf8");
const base = "https://racion.app";
const langs = Object.keys(seo);
const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

const alternates = (pathFor) =>
  langs.map((l) => `<link rel="alternate" hreflang="${l}" href="${base}${pathFor(l)}" />`).join("\n    ") +
  `\n    <link rel="alternate" hreflang="x-default" href="${base}/" />`;

function render(lang) {
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
  // текст для роботов без JS: заголовок, лид и ссылки на серверные страницы того же языка
  const list = links.map((l) => `          <li><a href="${l.href}">${esc(l.text)}</a></li>`).join("\n");
  html = html.replace(/<main class="prerender">[\s\S]*?<\/main>/,
    `<main class="prerender">\n        <h1>${esc(title.split(" — ")[0])}</h1>\n        <p>${esc(description)}</p>\n        <ul>\n${list}\n        </ul>\n      </main>`);
  return html;
}

for (const lang of langs) {
  const html = render(lang);
  if (lang === "ru") {
    fs.writeFileSync(path.join(dist, "index.html"), html);
    continue;
  }
  fs.mkdirSync(path.join(dist, lang), { recursive: true });
  fs.writeFileSync(path.join(dist, lang, "index.html"), html);
}
console.log("home pages:", langs.join(" "));
