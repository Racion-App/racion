import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { AlertCircle } from "lucide-react";
import { SiteFooter } from "../components/SiteFooter";
import { TopBar } from "../components/TopBar";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import { track } from "../lib/analytics";
import { useT } from "../i18n";

export function Login() {
  const [sp] = useSearchParams();
  const nav = useNavigate();
  const { setUser, refresh } = useAuth();
  const planId = sp.get("plan") ?? undefined;
  const next = sp.get("next") ?? "";
  const safeNext = next.startsWith("/") && !next.startsWith("//") ? next : "";
  const [mode, setMode] = useState<"login" | "register">(sp.get("mode") === "register" ? "register" : "login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const { t } = useT();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      const u = mode === "login" ? await api.login(email, password, planId) : await api.register(email, password, name, planId);
      setUser(u);
      void refresh(); // подтянуть флаг администратора
      track(mode === "login" ? "auth_login" : "auth_register");
      if (safeNext) {
        window.location.assign(safeNext); // SSR-страница рецепта живёт вне React-роутера
        return;
      }
      nav(planId ? `/plan/${planId}` : "/me", { replace: true });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell">
      <TopBar />
      <main className="auth">
        <div className="auth__card">
        <h1 className="auth__title">{mode === "login" ? t("auth.login") : t("auth.register")}</h1>
        <p className="auth__lead">{mode === "login" ? t("auth.login.lead") : t("auth.register.lead")}</p>
        <div className="segmented" role="radiogroup" aria-label={t("auth.mode")}>
          <button type="button" role="radio" aria-checked={mode === "login"} onClick={() => setMode("login")}>
            {t("auth.signin")}
          </button>
          <button type="button" role="radio" aria-checked={mode === "register"} onClick={() => setMode("register")}>
            {t("auth.create")}
          </button>
        </div>
        <form className="auth__form" onSubmit={submit}>
          {mode === "register" && (
            <label className="auth__field">
              <span>{t("auth.name")}</span>
              <input className="form-control" value={name} onChange={(e) => setName(e.target.value)} autoComplete="given-name" maxLength={80} placeholder={t("auth.name.placeholder")} />
            </label>
          )}
          <label className="auth__field">
            <span>{t("auth.email")}</span>
            <input className="form-control" type="email" required value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" inputMode="email" />
          </label>
          <label className="auth__field">
            <span>{t("auth.password")}{mode === "register" && <small> {t("auth.password.min")}</small>}</span>
            <input className="form-control" type="password" required minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} autoComplete={mode === "login" ? "current-password" : "new-password"} />
          </label>
          {mode === "register" && (
            <>
              <label className="auth__consent">
                <input type="checkbox" required />
                <span>{t("auth.consent")}</span>
              </label>
              <p className="auth__legal">
                {t("auth.legal.pre")} <a href="/terms" target="_blank" rel="noopener">{t("legal.terms")}</a> {t("auth.legal.and")} <a href="/privacy" target="_blank" rel="noopener">{t("legal.privacy")}</a>.
              </p>
            </>
          )}
          {error && (
            <p className="error-inline" role="alert">
              <AlertCircle size={18} aria-hidden /> {error}
            </p>
          )}
          <button type="submit" className="btn btn-primary btn-lg" disabled={busy}>
            {mode === "login" ? t("auth.signin") : t("auth.create")}
          </button>
          {planId && <p className="auth__foot">{t("auth.planSaved")}</p>}
        </form>
        <p className="auth__foot">
          {t("auth.noAccount", { link: "\u0000" }).split("\u0000")[0]}
          <Link to="/">{t("auth.noAccount.link")}</Link>
          {t("auth.noAccount", { link: "\u0000" }).split("\u0000")[1]}
        </p>
        </div>
      </main>
      <SiteFooter />
    </div>
  );
}
