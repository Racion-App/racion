import { BrowserRouter, Route, Routes } from "react-router-dom";
import { Quiz } from "./pages/Quiz";
import { Plan } from "./pages/Plan";
import { Login } from "./pages/Login";
import { Account } from "./pages/Account";
import { Admin } from "./pages/Admin";
import { NotFound } from "./pages/NotFound";
import { Occasion } from "./pages/Occasion";
import { AuthProvider } from "./lib/auth";
import { LangProvider } from "./i18n";
import { ConfirmProvider } from "./components/Confirm";

export function App() {
  return (
    <LangProvider>
    <AuthProvider>
      <ConfirmProvider>
      <BrowserRouter>
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
      </BrowserRouter>
      </ConfirmProvider>
    </AuthProvider>
    </LangProvider>
  );
}
