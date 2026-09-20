import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "flag-icons/css/flag-icons.min.css";
import "./styles/app.scss";
import { App } from "./App";
import { initAnalytics } from "./lib/analytics";

import { initOffline } from "./lib/offline";
initAnalytics();
initOffline();
try { window.addEventListener("load", () => setTimeout(() => sessionStorage.removeItem("racion.chunk.reload"), 5000)); } catch { /* приватный режим */ }
// офлайн-оболочка и список покупок: service worker регистрируется сразу, push подключается к нему же
if ("serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {});
  });
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
