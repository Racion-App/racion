// Тема до первой отрисовки: читает выбор из localStorage. Внешний файл, чтобы CSP обходилась без inline-скриптов.
try {
  var t = localStorage.getItem("racion.theme");
  if (t === "light" || t === "dark") document.documentElement.setAttribute("data-theme", t);
} catch (e) {}
