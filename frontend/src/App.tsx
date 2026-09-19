import { lazy, Suspense } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { Quiz } from "./pages/Quiz";
// Квиз — в основном бандле (главная), остальные страницы подгружаются по маршруту: первый экран легче
const Plan = lazy(() => import("./pages/Plan").then((m) => ({ default: m.Plan })));
const Login = lazy(() => import("./pages/Login").then((m) => ({ default: m.Login })));
const Account = lazy(() => import("./pages/Account").then((m) => ({ default: m.Account })));
const Admin = lazy(() => import("./pages/Admin").then((m) => ({ default: m.Admin })));
const NotFound = lazy(() => import("./pages/NotFound").then((m) => ({ default: m.NotFound })));
const Occasion = lazy(() => import("./pages/Occasion").then((m) => ({ default: m.Occasion })));
import { AuthProvider } from "./lib/auth";
import { LangProvider } from "./i18n";
import { ConfirmProvider } from "./components/Confirm";

export function App() {
  return (
    <LangProvider>
    <AuthProvider>
      <ConfirmProvider>
      <BrowserRouter>
        <Suspense fallback={<div className="boot" role="status" aria-busy="true"><span className="boot__spin" /></div>}>
        <Routes>
          <Route path="/" element={<Quiz />} />
          <Route path="/plan/:id" element={<Plan />} />
          <Route path="/login" element={<Login />} />
          <Route path="/me" element={<Account />} />
          <Route path="/admin" element={<Admin />} />
          <Route path="/admin/:tab" element={<Admin />} />
          <Route path="/admin/recipes/:id" element={<Admin />} />
          <Route path="/event/:id" element={<Occasion />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
        </Suspense>
      </BrowserRouter>
      </ConfirmProvider>
    </AuthProvider>
    </LangProvider>
  );
}
