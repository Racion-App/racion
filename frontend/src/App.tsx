import React, { Fragment, lazy, Suspense } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { Quiz } from "./pages/Quiz";
// Квиз — в основном бандле (главная), остальные страницы подгружаются по маршруту: первый экран легче.
// Имена чанков постоянные, поэтому после выкладки открытая старая вкладка может получить чанк новой сборки
// («does not provide an export named …»): тогда один раз перезагружаем страницу, дальше всё свежее.
function page<T>(load: () => Promise<T>, pick: (m: T) => React.ComponentType): React.LazyExoticComponent<React.ComponentType> {
  return lazy(() =>
    load().then((m) => ({ default: pick(m) })).catch((e: unknown) => {
      const key = "racion.chunk.reload";
      let again = false;
      try { again = sessionStorage.getItem(key) === "1"; if (!again) sessionStorage.setItem(key, "1"); } catch { /* приватный режим */ }
      if (!again) { location.reload(); return new Promise<never>(() => undefined); }
      throw e;
    }),
  );
}
const Plan = page(() => import("./pages/Plan"), (m) => m.Plan);
const Login = page(() => import("./pages/Login"), (m) => m.Login);
const Account = page(() => import("./pages/Account"), (m) => m.Account);
const Admin = page(() => import("./pages/Admin"), (m) => m.Admin);
const NotFound = page(() => import("./pages/NotFound"), (m) => m.NotFound);
const Occasion = page(() => import("./pages/Occasion"), (m) => m.Occasion);
const Cook = page(() => import("./pages/Cook"), (m) => m.Cook);
import { AuthProvider } from "./lib/auth";
import { LANG_PREFIXES, LangProvider } from "./i18n";
import { ConfirmProvider } from "./components/Confirm";
import { UpdateBar } from "./components/UpdateBar";

export function App() {
  return (
    <LangProvider>
    <AuthProvider>
      <ConfirmProvider>
      <BrowserRouter>
        <Suspense fallback={<div className="boot" role="status" aria-busy="true"><span className="boot__spin" /></div>}>
        <Routes>
          {/* те же экраны с языковым префиксом: /en, /de/plan/… — ссылка из другой локали не должна давать 404 */}
          {["", ...LANG_PREFIXES].map((p) => (
            <Fragment key={p || "root"}>
              <Route path={`${p}/`} element={<Quiz />} />
              <Route path={`${p}/plan/:id`} element={<Plan />} />
              <Route path={`${p}/login`} element={<Login />} />
              <Route path={`${p}/me`} element={<Account />} />
              <Route path={`${p}/admin`} element={<Admin />} />
              <Route path={`${p}/admin/:tab`} element={<Admin />} />
              <Route path={`${p}/admin/recipes/:id`} element={<Admin />} />
              <Route path={`${p}/event/:id`} element={<Occasion />} />
              <Route path={`${p}/cook/:id`} element={<Cook />} />
            </Fragment>
          ))}
          <Route path="*" element={<NotFound />} />
        </Routes>
        </Suspense>
        <UpdateBar />
      </BrowserRouter>
      </ConfirmProvider>
    </AuthProvider>
    </LangProvider>
  );
}
