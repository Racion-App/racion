// Уведомление о cookies для всех (закон о персональных данных) и выбор по аналитике для ЕС/ЕЭЗ и Британии:
// cookie racion_analytics=1|0 на год. Счётчики (inline в HTML) не стартуют при 0. Страна — из cookie racion_country или /api/meta.
(function () {
  if (document.cookie.indexOf("racion_analytics=") >= 0) return;
  var EU = "AT BE BG HR CY CZ DK EE FI FR DE GR HU IE IT LV LT LU MT NL PL PT RO SK SI ES SE IS LI NO GB CH".split(" ");
  var T = {
    ru: ["Cookies", "Сайт использует cookies для входа и настроек, а также Яндекс Метрику и Google Analytics, чтобы понимать, что работает.", "Разрешить", "Только необходимое", "/privacy", "Политика", "Понятно"],
    en: ["Cookies", "This site uses cookies for sign-in and settings, plus Yandex Metrica and Google Analytics to see what works.", "Allow", "Only necessary", "/privacy", "Privacy", "OK"],
    de: ["Analyse", "Wir nutzen Yandex Metrica und Google Analytics, um zu sehen, was funktioniert. Du kannst ablehnen — der Dienst funktioniert genauso.", "Erlauben", "Nur notwendige", "/de/privacy", "Datenschutz", "Verstanden"],
    fr: ["Statistiques", "Nous utilisons Yandex Metrica et Google Analytics pour voir ce qui fonctionne. Vous pouvez refuser — le service fonctionne pareil.", "Autoriser", "Nécessaires seulement", "/fr/privacy", "Confidentialité", "D’accord"],
    es: ["Analítica", "Usamos Yandex Metrica y Google Analytics para ver qué funciona. Puedes rechazar — el servicio funciona igual.", "Permitir", "Solo necesarias", "/es/privacy", "Privacidad", "Entendido"],
    it: ["Statistiche", "Usiamo Yandex Metrica e Google Analytics per capire cosa funziona. Puoi rifiutare — il servizio funziona lo stesso.", "Consenti", "Solo necessari", "/it/privacy", "Privacy", "Capito"],
    pl: ["Analityka", "Używamy Yandex Metrica i Google Analytics, by widzieć, co działa. Możesz odmówić — serwis działa tak samo.", "Zezwól", "Tylko niezbędne", "/pl/privacy", "Prywatność", "Rozumiem"],
    nl: ["Analyse", "We gebruiken Yandex Metrica en Google Analytics om te zien wat werkt. Je kunt weigeren — de dienst werkt hetzelfde.", "Toestaan", "Alleen noodzakelijk", "/nl/privacy", "Privacy", "Begrepen"],
    cs: ["Analytika", "Používáme Yandex Metrica a Google Analytics, abychom viděli, co funguje. Můžete odmítnout — služba funguje stejně.", "Povolit", "Jen nezbytné", "/cs/privacy", "Soukromí", "Rozumím"],
    pt: ["Análise", "Usamos Yandex Metrica e Google Analytics para ver o que funciona. Pode recusar — o serviço funciona igual.", "Permitir", "Só necessários", "/pt/privacy", "Privacidade", "Entendi"],
    uk: ["Аналітика", "Ми використовуємо Yandex Metrica і Google Analytics, щоб бачити, що працює. Можна відмовитися — сервіс працює так само.", "Дозволити", "Лише необхідні", "/uk/privacy", "Політика", "Зрозуміло"],
    tr: ["Analitik", "Neyin işe yaradığını görmek için Yandex Metrica ve Google Analytics kullanıyoruz. Reddedebilirsiniz, hizmet aynı şekilde çalışır.", "İzin ver", "Yalnızca gerekli", "/tr/privacy", "Gizlilik", "Anladım"],
    kk: ["Аналитика", "Не жұмыс істейтінін көру үшін Yandex Metrica мен Google Analytics қолданамыз. Бас тартуға болады, қызмет солай жұмыс істейді.", "Рұқсат ету", "Тек қажеттісі", "/kk/privacy", "Құпиялылық", "Түсінікті"],
    zh: ["分析", "我们使用 Yandex Metrica 和 Google Analytics 了解哪些功能有用。你可以拒绝，服务照常使用。", "允许", "仅必要项", "/zh/privacy", "隐私政策", "知道了"],
    ja: ["アクセス解析", "何が使われているかを見るために Yandex Metrica と Google Analytics を使っています。断っても機能は変わりません。", "許可する", "必要なものだけ", "/ja/privacy", "プライバシー", "わかりました"]
  };
  // язык: префикс адреса → выбранный в приложении → язык браузера → атрибут html (в index.html он всегда ru,
  // поэтому последним). Префикс первый: на /en/plan/… отдаётся русская оболочка приложения, и без него
  // англоязычный посетитель видел уведомление по-русски.
  function lang() {
    var p = (location.pathname.split("/")[1] || "").toLowerCase();
    if (/^[a-z]{2}$/.test(p) && T[p]) return p;
    var s = "";
    try { s = (localStorage.getItem("racion.lang") || "").replace(/"/g, ""); } catch (e) { /* приватный режим */ }
    if (T[s]) return s;
    var m = document.documentElement.lang || "";
    if (T[m] && m !== "ru") return m;
    var b = ((navigator.language || "").split("-")[0] || "").toLowerCase();
    if (T[b]) return b;
    return T[m] ? m : "en";
  }
  function set(v) { document.cookie = "racion_analytics=" + v + "; path=/; max-age=31536000; SameSite=Lax"; }
  function show(eu) {
    var t = T[lang()], d = document.createElement("div");
    d.className = "consent"; d.setAttribute("role", "dialog"); d.setAttribute("aria-label", t[0]);
    // ЕС: разрешить / только необходимое; остальные: уведомление с одной кнопкой
    var btns = eu
      ? '<button type="button" class="btn btn-soft btn-sm" data-c="0">' + t[3] + '</button><button type="button" class="btn btn-primary btn-sm" data-c="1">' + t[2] + "</button>"
      : '<button type="button" class="btn btn-primary btn-sm" data-c="1">' + t[6] + "</button>";
    d.innerHTML = '<div class="consent__text"><b>' + t[0] + "</b> " + t[1] + ' <a href="' + t[4] + '">' + t[5] + "</a></div>" + '<div class="consent__btns">' + btns + "</div>";
    d.addEventListener("click", function (e) {
      var b = e.target.closest("[data-c]"); if (!b) return;
      set(b.getAttribute("data-c")); d.remove();
    });
    document.body.appendChild(d);
  }
  function country(cb) {
    var m = document.cookie.match(/(?:^|; )racion_country=([A-Z]{2})/);
    if (m) return cb(m[1]);
    fetch("/api/meta", { credentials: "same-origin" }).then(function (r) { return r.json(); }).then(function (j) { cb(j.geoCountry || ""); }).catch(function () { cb(""); });
  }
  country(function (c) { show(EU.indexOf(c) >= 0); });
})();
