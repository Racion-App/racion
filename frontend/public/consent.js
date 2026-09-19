// Выбор по аналитике для посетителей из ЕС/ЕЭЗ и Британии: cookie racion_analytics=1|0 на год.
// Счётчики (inline в HTML) не стартуют при значении 0. Страна — из cookie racion_country или /api/meta.
(function () {
  if (document.cookie.indexOf("racion_analytics=") >= 0) return;
  var EU = "AT BE BG HR CY CZ DK EE FI FR DE GR HU IE IT LV LT LU MT NL PL PT RO SK SI ES SE IS LI NO GB CH".split(" ");
  var T = {
    ru: ["Аналитика", "Мы используем Яндекс Метрику и Google Analytics, чтобы понимать, что работает. Можно отказаться — сервис будет работать так же.", "Разрешить", "Только необходимое", "/privacy", "Политика"],
    en: ["Analytics", "We use Yandex Metrica and Google Analytics to see what works. You can decline — the service works the same.", "Allow", "Only necessary", "/privacy", "Privacy"],
    de: ["Analyse", "Wir nutzen Yandex Metrica und Google Analytics, um zu sehen, was funktioniert. Du kannst ablehnen — der Dienst funktioniert genauso.", "Erlauben", "Nur notwendige", "/de/privacy", "Datenschutz"],
    fr: ["Statistiques", "Nous utilisons Yandex Metrica et Google Analytics pour voir ce qui fonctionne. Vous pouvez refuser — le service fonctionne pareil.", "Autoriser", "Nécessaires seulement", "/fr/privacy", "Confidentialité"],
    es: ["Analítica", "Usamos Yandex Metrica y Google Analytics para ver qué funciona. Puedes rechazar — el servicio funciona igual.", "Permitir", "Solo necesarias", "/es/privacy", "Privacidad"],
    it: ["Statistiche", "Usiamo Yandex Metrica e Google Analytics per capire cosa funziona. Puoi rifiutare — il servizio funziona lo stesso.", "Consenti", "Solo necessari", "/it/privacy", "Privacy"],
    pl: ["Analityka", "Używamy Yandex Metrica i Google Analytics, by widzieć, co działa. Możesz odmówić — serwis działa tak samo.", "Zezwól", "Tylko niezbędne", "/pl/privacy", "Prywatność"],
    nl: ["Analyse", "We gebruiken Yandex Metrica en Google Analytics om te zien wat werkt. Je kunt weigeren — de dienst werkt hetzelfde.", "Toestaan", "Alleen noodzakelijk", "/nl/privacy", "Privacy"],
    cs: ["Analytika", "Používáme Yandex Metrica a Google Analytics, abychom viděli, co funguje. Můžete odmítnout — služba funguje stejně.", "Povolit", "Jen nezbytné", "/cs/privacy", "Soukromí"],
    pt: ["Análise", "Usamos Yandex Metrica e Google Analytics para ver o que funciona. Pode recusar — o serviço funciona igual.", "Permitir", "Só necessários", "/pt/privacy", "Privacidade"]
  };
  function lang() {
    var m = document.documentElement.lang || "";
    if (T[m]) return m;
    var s = (localStorage.getItem("racion.lang") || "").replace(/"/g, "");
    return T[s] ? s : "en";
  }
  function set(v) { document.cookie = "racion_analytics=" + v + "; path=/; max-age=31536000; SameSite=Lax"; }
  function show() {
    var t = T[lang()], d = document.createElement("div");
    d.className = "consent"; d.setAttribute("role", "dialog"); d.setAttribute("aria-label", t[0]);
    d.innerHTML = '<div class="consent__text"><b>' + t[0] + "</b> " + t[1] + ' <a href="' + t[4] + '">' + t[5] + "</a></div>" +
      '<div class="consent__btns"><button type="button" class="btn btn-soft btn-sm" data-c="0">' + t[3] + '</button><button type="button" class="btn btn-primary btn-sm" data-c="1">' + t[2] + "</button></div>";
    d.addEventListener("click", function (e) {
      var b = e.target.closest("[data-c]"); if (!b) return;
      set(b.getAttribute("data-c")); d.remove();
      if (b.getAttribute("data-c") === "1") location.reload();
    });
    document.body.appendChild(d);
  }
  function country(cb) {
    var m = document.cookie.match(/(?:^|; )racion_country=([A-Z]{2})/);
    if (m) return cb(m[1]);
    fetch("/api/meta", { credentials: "same-origin" }).then(function (r) { return r.json(); }).then(function (j) { cb(j.geoCountry || ""); }).catch(function () { cb(""); });
  }
  country(function (c) { if (EU.indexOf(c) >= 0) show(); else set("1"); });
})();
