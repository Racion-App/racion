import type React from "react";
import { useEffect, useState } from "react";
import { Heart, MessageCircle, Share2, ThumbsUp } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";
import type { RecipeStats } from "../lib/types";
import { useT } from "../i18n";

// Лайк, избранное, комментарии и «поделиться» у рецепта в листе. Гость по лайку уходит на вход и возвращается.

export function SocialBar({ recipeId, initial, onToast, extra }: { recipeId: string; initial?: RecipeStats; onToast: (m: string) => void; extra?: React.ReactNode }) {
  const { t, lang } = useT();
  const { user } = useAuth();
  const nav = useNavigate();
  const [st, setSt] = useState<RecipeStats>(initial ?? { likes: 0, comments: 0, liked: false, favorite: false });
  useEffect(() => {
    if (initial) setSt(initial);
  }, [initial, recipeId]);

  const pageHref = `${lang === "ru" ? "" : `/${lang}`}/recipe/${recipeId}`;
  const guard = () => {
    if (user) return true;
    nav(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    return false;
  };
  const like = async () => {
    if (!guard()) return;
    try {
      setSt(await api.like(recipeId, !st.liked));
    } catch (e) {
      onToast((e as Error).message);
    }
  };
  const fav = async () => {
    if (!guard()) return;
    try {
      const next = await api.favorite(recipeId, !st.favorite);
      setSt(next);
      onToast(next.favorite ? t("social.infav") : t("social.unfav.done"));
    } catch (e) {
      onToast((e as Error).message);
    }
  };
  const share = async () => {
    const url = window.location.origin + pageHref;
    try {
      if (navigator.share) {
        await navigator.share({ url });
        return;
      }
      await navigator.clipboard.writeText(url);
      onToast(t("social.copied"));
    } catch {
      /* отменили */
    }
  };

  return (
    <div className="social social--sheet">
      <button type="button" className="social__btn" aria-pressed={st.liked} aria-label={st.liked ? t("social.unlike") : t("social.like")} onClick={like}>
        <ThumbsUp size={16} aria-hidden /> <span className="num">{st.likes}</span>
      </button>
      <button type="button" className="social__btn" aria-pressed={st.favorite} aria-label={st.favorite ? t("social.unfav") : t("social.fav")} onClick={fav}>
        <Heart size={16} aria-hidden /> <span>{st.favorite ? t("social.infav") : t("social.fav")}</span>
      </button>
      <a className="social__btn" href={`${pageHref}#comments`}>
        <MessageCircle size={16} aria-hidden /> <span className="num">{st.comments}</span>
      </a>
      {extra}
      <button type="button" className="social__btn" onClick={share} aria-label={t("social.share")}>
        <Share2 size={16} aria-hidden /> <span>{t("social.share")}</span>
      </button>
    </div>
  );
}
