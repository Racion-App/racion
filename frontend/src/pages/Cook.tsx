import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { CookMode } from "../components/CookMode";
import { api } from "../lib/api";
import { readDraft } from "../lib/draft";
import type { Recipe } from "../lib/types";
import { useT } from "../i18n";

// /cook/{id}: режим готовки для любого рецепта — со страницы рецепта (SSR) и по прямой ссылке,
// не только из недели. Порции — взрослые из черновика квиза, иначе одна. Закрытие возвращает
// на страницу рецепта.

export function Cook() {
  const { id = "" } = useParams();
  const nav = useNavigate();
  const { t, lang } = useT();
  const [recipe, setRecipe] = useState<Recipe | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    api
      .recipe(id)
      .then((r) => alive && setRecipe(r))
      .catch((e: Error) => alive && setError(e.message));
    return () => {
      alive = false;
    };
  }, [id, lang]);
  const close = () => {
    if (window.history.length > 1 && document.referrer.includes(location.host)) nav(-1);
    else location.assign(`/recipe/${encodeURIComponent(id)}`);
  };
  if (error) {
    return (
      <div className="state">
        <div className="state__box">
          <h2>{t("page.404.title")}</h2>
          <a className="btn btn-primary" href="/recipes">{t("page.foot.recipes")}</a>
        </div>
      </div>
    );
  }
  if (!recipe) {
    return <div className="boot" role="status" aria-busy="true"><span className="boot__spin" /></div>;
  }
  const portions = Math.max(1, readDraft().adults ?? 1);
  return <CookMode recipe={recipe} portions={portions} onClose={close} />;
}
