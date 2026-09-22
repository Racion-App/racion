// «Мой стол» на серверных страницах: кнопка на карточке рецепта кладёт блюдо в localStorage,
// внизу появляется полоса со счётчиком и ссылкой на /table, где стол превращается в список покупок.
// Приложение читает то же хранилище, поэтому страница стола — обычный экран React.
(function () {
  var KEY = "racion.table";

  function read() {
    try {
      var v = JSON.parse(localStorage.getItem(KEY) || "[]");
      return Array.isArray(v) ? v.filter(function (x) { return x && typeof x.id === "string"; }) : [];
    } catch (e) {
      return []; // приватный режим или битое хранилище: стол просто пустой
    }
  }

  function write(list) {
    try {
      localStorage.setItem(KEY, JSON.stringify(list));
    } catch (e) { /* хранилище недоступно — кнопки останутся, но стол не переживёт перезагрузку */ }
  }

  function has(list, id) {
    return list.some(function (x) { return x.id === id; });
  }

  function label(el, on) {
    var add = el.getAttribute("data-l-add") || "";
    var added = el.getAttribute("data-l-added") || add;
    el.classList.toggle("is-on", on);
    el.setAttribute("aria-pressed", on ? "true" : "false");
    var span = el.querySelector("[data-table-label]");
    if (span) span.textContent = on ? added : add;
    else el.setAttribute("aria-label", on ? added : add); // кнопка без подписи (старая разметка)
  }

  function paint() {
    var list = read();
    var btns = document.querySelectorAll("[data-table-add]");
    for (var i = 0; i < btns.length; i++) {
      label(btns[i], has(list, btns[i].getAttribute("data-table-add")));
    }
    var bar = document.querySelector("[data-table-bar]");
    if (!bar) return;
    var n = list.length;
    bar.hidden = n === 0;
    var count = bar.querySelector("[data-table-count]");
    if (count) {
      var fmt = bar.getAttribute("data-l-count") || "{n}";
      count.textContent = fmt.replace("{n}", String(n));
    }
  }

  document.addEventListener("click", function (e) {
    var btn = e.target.closest ? e.target.closest("[data-table-add]") : null;
    if (!btn) return;
    e.preventDefault();
    var id = btn.getAttribute("data-table-add");
    var list = read();
    if (has(list, id)) {
      list = list.filter(function (x) { return x.id !== id; });
    } else if (list.length < 30) {
      list.push({ id: id, servings: 0 }); // 0 — «как у стола», человек поправит на странице стола
    }
    write(list);
    paint();
  });

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", paint);
  } else {
    paint();
  }
  window.addEventListener("pageshow", paint); // возврат «назад» из кэша браузера
  window.addEventListener("storage", paint);  // стол поменяли в другой вкладке
})();
