// Логотип сети на плитке квиза. Три источника: официальный SVG (frontend/public/logos, см. README там),
// тот же файл для той же сети в другой стране (ALIAS), иначе — монограмма в фирменном цвете сети.

export const WITH_LOGO = new Set([
  "pyaterochka", "magnit", "dixy", "lenta", "auchan", "perekrestok", "metro", "vkusvill", "samokat",
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
    return <img className="store-logo" src={`/logos/${file}.svg`} alt="" aria-hidden="true" draggable={false} width={96} height={24} />;
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
  // Азбука Вкуса: официальный знак — квадратная иконка без названия, поэтому рядом пишем название, как у монограмм.
  if (code === "azbukavkusa") {
    return (
      <span className="store-logo store-logo--drawn" aria-hidden="true">
        <img className="store-logo__icon" src="/logos/azbukavkusa.svg" alt="" draggable={false} width={26} height={26} />
        <span className="store-logo__text">{name}</span>
      </span>
    );
  }
  // Яндекс Лавка: официальный логотип (logos/lavka.svg) встроен, чтобы слово «Лавка» брало цвет текста темы —
  // в файле оно почти чёрное и пропадает на тёмной плитке. Круги «Я» и сердце — фирменных цветов.
  if (code === "lavka") {
    return (
      <span className="store-logo store-logo--inline" aria-hidden="true">
        <svg width="96" height="19" viewBox="0 0 131 26" role="img" focusable="false">
          <path d="M26 13C26 5.8203 20.1797 0 13 0C5.8203 0 0 5.8203 0 13C0 20.1797 5.8203 26 13 26C20.1797 26 26 20.1797 26 13Z" fill="#FC3F1D" />
          <path d="M14.832 20.813H17.548V5.213H13.598C9.625 5.213 7.538 7.256 7.538 10.263C7.538 12.665 8.682 14.079 10.725 15.538L7.178 20.813H10.118L14.068 14.91L12.7 13.99C11.04 12.867 10.23 11.992 10.23 10.106C10.23 8.446 11.398 7.323 13.62 7.323H14.832V20.813Z" fill="#fff" />
          <path d="M58.5 20.053V23.39C58.5 23.39 59.016 23.693 60.017 23.693C63.596 23.693 64.597 20.751 64.779 13.046L64.991 5.159H70.148V23.389H73.818V2.31H61.685L61.412 13.107C61.291 18.233 61.018 20.327 59.532 20.327C58.803 20.326 58.5 20.053 58.5 20.053ZM88.266 12.985C88.266 9.285 86.386 7.889 82.564 7.889C80.168 7.889 78.287 8.649 77.194 9.285V12.288C78.164 11.56 80.289 10.771 82.139 10.771C83.865 10.771 84.654 11.378 84.654 13.016V13.866H84.078C78.556 13.866 76.098 15.686 76.098 18.779C76.099 21.873 77.979 23.602 80.769 23.602C82.893 23.602 83.804 22.904 84.501 22.176H84.653C84.683 22.571 84.803 23.086 84.926 23.39H88.444C88.3232 22.1503 88.2628 20.9055 88.263 19.66V12.985H88.266ZM84.656 19.75C84.202 20.417 83.353 20.963 82.078 20.963C80.562 20.963 79.803 20.053 79.803 18.688C79.803 16.898 81.047 16.261 84.141 16.261H84.66V19.751L84.656 19.75ZM97.175 23.39C100.755 23.39 102.877 21.873 102.877 18.96C102.877 16.959 101.664 15.806 99.632 15.442C101.27 14.987 102.301 13.835 102.301 12.015C102.301 9.406 100.571 8.132 97.235 8.132H91.17V23.39H97.175ZM96.72 10.862C98.024 10.862 98.752 11.408 98.752 12.561C98.752 13.622 97.963 14.259 96.598 14.259H94.778V10.862H96.72ZM96.78 16.898C98.327 16.898 99.146 17.444 99.146 18.718C99.146 20.114 98.236 20.66 96.78 20.66H94.778V16.898H96.78ZM114.247 23.39H118.342L112.548 15.17L117.644 8.132H114.004L108.908 15.169V8.132H105.299V23.39H108.908V15.897L114.247 23.39ZM130.627 12.985C130.627 9.285 128.746 7.889 124.924 7.889C122.528 7.889 120.647 8.649 119.555 9.285V12.288C120.525 11.56 122.649 10.771 124.499 10.771C126.225 10.771 127.014 11.378 127.014 13.016V13.866H126.438C120.918 13.866 118.459 15.686 118.459 18.779C118.459 21.873 120.34 23.602 123.13 23.602C125.254 23.602 126.164 22.904 126.86 22.176H127.013C127.043 22.571 127.165 23.086 127.286 23.39H130.806C130.684 22.1504 130.623 20.9056 130.623 19.66V12.985H130.627ZM127.017 19.75C126.562 20.417 125.713 20.963 124.439 20.963C122.922 20.963 122.164 20.053 122.164 18.688C122.164 16.898 123.407 16.261 126.501 16.261H127.017V19.75Z" fill="currentColor" />
          <path d="M54.166 13C54.166 5.82 48.346 0 41.166 0C33.986 0 28.166 5.82 28.166 13C28.166 20.18 33.986 26 41.166 26C48.346 26 54.166 20.18 54.166 13Z" fill="#00ADFF" />
          <path d="M41.688 13.223C42.721 8.383 43.663 6.618 45.529 6.618C48.626 6.618 50.017 9.468 48.049 15.313C46.152 20.947 41.972 21.949 38.935 20.148C36.732 18.842 34.995 16.021 34.338 14.464C33.303 12.018 32.76 9.344 34.122 7.884C35.237 6.689 36.519 6.113 38.935 8.859C41.351 11.605 41.685 13.223 41.685 13.223H41.688Z" fill="#fff" />
        </svg>
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
