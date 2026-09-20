// Service worker «Рациона»: push-уведомления и офлайн-список покупок.
// Кэш: оболочка приложения (index, скрипты, стили, шрифты, иконки) и последние открытые недели
// (/api/plans/{id} и их отметки), чтобы на кассе без сети список открывался и отмечался.
// Отметки, сделанные офлайн, приложение копит в localStorage и досылает, когда сеть вернётся.
const VERSION = "racion-v6";
const SHELL = ["/", "/offline.html", "/theme.js", "/assets/app.js", "/assets/app.css", "/manifest.webmanifest", "/favicon.svg", "/fonts/InterVariable.woff2", "/fonts/JetBrainsMono-Medium.woff2", "/icons/icon-192.png", "/icons/icon-512.png", "/icons/badge-72.png"];
// маршруты приложения без сервера: их открывает SPA из кэша; остальное (рецепты, подборки) — страница «нет сети»
const APP_RE = /^\/([a-z]{2}\/)?(plan\/|me|login|event\/|cook\/|$|\?)/;
const PLAN_RE = /^\/api\/(plans\/[^/]+(\/checks|\/extras)?|recipes\/[^/]+(\/subs|\/stats)?|me(\/plans)?|locales(\/[a-z]{2})?|meta)(\?.*)?$/;

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(VERSION).then((c) => c.addAll(SHELL).catch(() => {})).then(() => self.skipWaiting()));
});
self.addEventListener("activate", (e) => {
  e.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== VERSION).map((k) => caches.delete(k)))).then(() => self.clients.claim()));
});

self.addEventListener("fetch", (e) => {
  const req = e.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== location.origin) return;
  // страницы приложения: сеть, при обрыве — оболочка из кэша (SPA сама покажет план из кэша)
  if (req.mode === "navigate") {
    e.respondWith(
      fetch(req)
        .then((res) => { if (url.pathname === "/") { const copy = res.clone(); caches.open(VERSION).then((c) => c.put("/", copy)).catch(() => {}); } return res; })
        .catch(() => (APP_RE.test(url.pathname + url.search) ? caches.match("/") : caches.match("/offline.html")).then((hit) => hit || caches.match("/"))),
    );
    return;
  }
  // скрипты и стили приложения без хеша в имени: сеть первой (после выкладки — свежие), кэш — только офлайн
  if (url.pathname.startsWith("/assets/")) {
    e.respondWith(fetch(req).then((res) => { if (res.ok) { const copy = res.clone(); caches.open(VERSION).then((c) => c.put(req, copy)).catch(() => {}); } return res; }).catch(() => caches.match(req)));
    return;
  }
  // шрифты, иконки, фото: из кэша, добираем при первом обращении
  if (url.pathname.startsWith("/fonts/") || url.pathname.startsWith("/icons/") || url.pathname === "/favicon.svg" || url.pathname.startsWith("/images/") || url.pathname.startsWith("/media/") || url.pathname.startsWith("/logos/")) {
    e.respondWith(caches.match(req).then((hit) => hit || fetch(req).then((res) => { if (res.ok) { const copy = res.clone(); caches.open(VERSION).then((c) => c.put(req, copy)).catch(() => {}); } return res; })));
    return;
  }
  // недели, отметки, словари: сеть первой, кэш — если сети нет
  if (PLAN_RE.test(url.pathname + url.search)) {
    e.respondWith(fetch(req).then((res) => { if (res.ok) { const copy = res.clone(); caches.open(VERSION).then((c) => c.put(req, copy)).catch(() => {}); } return res; }).catch(() => caches.match(req).then((hit) => hit || new Response(JSON.stringify({ error: "offline" }), { status: 503, headers: { "Content-Type": "application/json" } }))));
  }
});

self.addEventListener("push", (e) => {
  let data = { title: "Рацион", body: "", url: "/", tag: "racion", actions: [], recipe: "" };
  try {
    data = { ...data, ...e.data.json() };
  } catch (err) {
    if (e.data) data.body = e.data.text();
  }
  // открытым вкладкам сообщаем, что пуш дошёл: кнопка «Проверить» в кабинете по этому понимает, где обрыв
  self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((list) => list.forEach((c) => c.postMessage({ type: "push", tag: data.tag }))).catch(() => {});
  e.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      tag: data.tag,
      icon: "/icons/icon-192.png",
      badge: "/icons/badge-72.png",
      actions: (data.actions || []).map((a) => ({ action: a.action, title: a.title })),
      data: { url: data.url, recipe: data.recipe },
      renotify: false,
    }),
  );
});

// Кнопки «понравилось» / «не зашло» отвечают прямо из уведомления, cookie сессии уходит сама (same-origin).
self.addEventListener("notificationclick", (e) => {
  e.notification.close();
  const d = e.notification.data || {};
  if ((e.action === "like" || e.action === "meh") && d.recipe) {
    e.waitUntil(
      fetch("/api/recipes/" + encodeURIComponent(d.recipe) + "/feedback", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ liked: e.action === "like" }),
        credentials: "same-origin",
      }).catch(() => {}),
    );
    return;
  }
  const url = d.url || "/";
  e.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((list) => {
      for (const c of list) {
        if ("focus" in c) {
          c.navigate(url);
          return c.focus();
        }
      }
      return self.clients.openWindow(url);
    }),
  );
});
