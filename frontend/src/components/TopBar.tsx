import { useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { Moon, MonitorSmartphone, ReceiptText, Sun, UserRound } from "lucide-react";
import { useAuth } from "../lib/auth";
import { getTheme, setTheme, THEME_NEXT, type Theme } from "../lib/theme";
import { useT } from "../i18n";
import { LangButton } from "./LangDialog";

function ThemeButton() {
  const { t } = useT();
  const [theme, set] = useState<Theme>(() => getTheme());
  const next = THEME_NEXT[theme];
  const Icon = theme === "light" ? Sun : theme === "dark" ? Moon : MonitorSmartphone;
  const label = t("theme", { cur: t(`theme.${theme}`), next: t(`theme.${next}`) });
  return (
    <button
      type="button"
      className="theme-btn"
      onClick={() => {
        setTheme(next);
        set(next);
      }}
      aria-label={label}
      title={t(`theme.${theme}`)}
    >
      <Icon size={18} aria-hidden />
    </button>
  );
}

export function TopBar({ right }: { right?: ReactNode }) {
  const { user } = useAuth();
  const { t, lang } = useT();
  const prefix = lang === "ru" ? "" : `/${lang}`;
  return (
    <header className={"topbar" + (right ? " topbar--busy" : "")}>
      <Link to="/" className="topbar__brand" aria-label={t("brand.home")}>
        <ReceiptText size={22} strokeWidth={2.2} aria-hidden />
        {t("brand")}
      </Link>
      <div className="topbar__right">
        {right}
        <a href={`${prefix}/recipes`} className="pages-nav__link">
          {t("nav.recipes")}
        </a>
        <LangButton />
        <Link to={user ? "/me" : "/login"} className={"theme-btn" + (user ? " is-user" : "")} aria-label={user ? t("nav.account") : t("nav.login")} title={user ? t("nav.account") : t("nav.login")}>
          <UserRound size={18} aria-hidden />
        </Link>
        <ThemeButton />
      </div>
    </header>
  );
}
