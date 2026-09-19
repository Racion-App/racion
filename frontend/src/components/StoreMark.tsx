// Логотип сети на плитке квиза. Три источника: официальный SVG (frontend/public/logos, см. README там),
// тот же файл для той же сети в другой стране (ALIAS), иначе — монограмма в фирменном цвете сети.

export const WITH_LOGO = new Set([
  "pyaterochka", "magnit", "dixy", "lenta", "auchan", "perekrestok", "metro", "vkusvill",
  "magnum", "aldi", "lidl", "penny", "netto", "rewe", "edeka", "kaufland",
  "walmart", "aldi_us", "costco", "target", "traderjoes", "safeway", "wholefoods",
  "kroger", "tesco", "sainsburys", "asda", "morrisons", "waitrose",
  "hofer", "billa", "spar_at", "intermarche", "monoprix", "leclerc",
  "coop_it", "esselunga", "eurospin", "mercadona", "dia", "eroski",
  "continente", "pingodoce", "minipreco", "albertheijn", "jumbo", "plus", "colruyt", "delhaize",
  "prisma", "smarket", "kcitymarket", "ksupermarket", "maxima_lt", "iki", "norfa", "coop_ee", "zabka",
  "ica", "coop_se", "willys", "hemkop", "rema", "extra", "coop_no", "migros", "coop_ch", "denner",
]);

// Одна сеть — один файл: Lidl в Польше выглядит как Lidl в Германии.
const ALIAS: Record<string, string> = {
  lidl_gb: "lidl", lidl_at: "lidl", lidl_fr: "lidl", lidl_it: "lidl", lidl_es: "lidl", lidl_pt: "lidl", lidl_nl: "lidl", lidl_be: "lidl",
  lidl_fi: "lidl", lidl_lt: "lidl", lidl_lv: "lidl", lidl_ee: "lidl", lidl_pl: "lidl", lidl_cz: "lidl", lidl_se: "lidl", lidl_ch: "lidl",
  aldi_gb: "aldi", aldi_nl: "aldi", aldi_be: "aldi", aldi_ch: "aldi",
  penny_at: "penny", penny_cz: "penny", kaufland_pl: "kaufland", kaufland_cz: "kaufland",
  auchan_fr: "auchan", auchan_pt: "auchan", auchan_pl: "auchan", metro_kz: "metro",
  billa_cz: "billa", tesco_cz: "tesco", mercadona_pt: "mercadona",
  carrefour_it: "carrefour", carrefour_es: "carrefour", carrefour_be: "carrefour", carrefour_pl: "carrefour",
  rimi_lv: "rimi_lt", rimi_ee: "rimi_lt", maxima_lv: "maxima_lt", maxima_ee: "maxima_lt", prisma_ee: "prisma",
};

// Фирменный цвет для монограммы там, где свободного SVG нет (белорусские и казахстанские сети, Carrefour, Biedronka…).
const BRAND: Record<string, string> = {
  svetofor: "#2B2B2B", evroopt: "#E2001A", gippo: "#F39200", green: "#3AAA35", korona: "#D71920", santa: "#0066B3", dobronom: "#E30613", vitalur: "#00843D",
  small: "#E4002B", galmart: "#7B2D8E", anvar: "#E31E24", arzan: "#F7A600", metro_kz: "#003D7C",
  carrefour: "#004E9F", conad: "#F7A600", alcampo: "#E30613", interspar: "#EE1C25", dirk: "#E30613", rimi_lt: "#E2001A",
  top: "#D40000", selver: "#E4002B", biedronka: "#D6001C", dino: "#E30613", albert: "#E2001A", kiwi: "#008A3F", meny: "#E4002B",
};

// Монограмма: первые буквы слов (до двух) в фирменном цвете; для Żabka/Ż и кириллицы работает так же.
function initials(name: string): string {
  const words = name.replace(/[^\p{L}\p{N} ]/gu, " ").trim().split(/\s+/).filter(Boolean);
  const s = words.length >= 2 ? words[0][0] + words[1][0] : (words[0] ?? name).slice(0, 2);
  return s.toUpperCase();
}

export function StoreMark({ code, name }: { code: string; name: string }) {
  const file = WITH_LOGO.has(code) ? code : ALIAS[code] && WITH_LOGO.has(ALIAS[code]) ? ALIAS[code] : null;
  if (file) {
    return <img className="store-logo" src={`/logos/${file}.svg`} alt="" aria-hidden="true" draggable={false} />;
  }
  if (code === "svetofor") {
    return (
      <span className="store-logo store-logo--drawn" aria-hidden="true">
        <svg width="16" height="26" viewBox="0 0 16 26" role="img" focusable="false">
          <rect x="1" y="1" width="14" height="24" rx="4" fill="#2B2B2B" />
          <circle cx="8" cy="6.5" r="3" fill="#F03A2F" />
          <circle cx="8" cy="13" r="3" fill="#F5C518" />
          <circle cx="8" cy="19.5" r="3" fill="#3BB54A" />
        </svg>
        <span className="store-logo__text">{name}</span>
      </span>
    );
  }
  const color = BRAND[code] ?? "#5B6B7A";
  return (
    <span className="store-logo store-logo--drawn" aria-hidden="true">
      <span className="store-logo__mono" style={{ background: color }}>
        {initials(name)}
      </span>
      <span className="store-logo__text">{name}</span>
    </span>
  );
}
