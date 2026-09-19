import { useEffect, useState, type FormEvent } from "react";
import { Copy, KeyRound, Plus, Trash2 } from "lucide-react";
import { api } from "../lib/api";
import type { ApiKey } from "../lib/types";
import { useConfirm } from "./Confirm";
import { dateShort } from "../lib/format";
import { useT } from "../i18n";

// Вкладка «API»: ключи и инструкция, как наполнять базу через помощника (Claude, Codex) или скриптом.
// Инструкция намеренно по-русски: это внутренний инструмент редакции. Свежий ключ подставляется в промпт.

const BASE = typeof location !== "undefined" ? location.origin : "https://racion.app";

const PROMPT = (key: string) => `Ты добавляешь рецепты в базу сервиса Racion через его API.
Базовый адрес: ${BASE}
Заголовок для всех запросов: Authorization: Bearer ${key}

Порядок работы:
1. GET /api/admin/recipes/schema — прочитай допустимые slot, equipment, tags и правила.
2. GET /api/admin/ingredients?q=слово — для каждого продукта найди id в базе (поиск по русскому, английскому названию или id). Продуктов, которых нет в базе, в рецепт не ставить: подбери ближайший из базы или пропусти рецепт и скажи мне, чего не хватает.
3. Собери рецепты по схеме: id (латиница и _), title, description, slot, timeMin, batch, equipment[], tags[] (только из схемы), steps[] (повелительное наклонение, с температурой и временем), ingredients[] = [{ingredientId, amount}] — amount на ОДНУ порцию взрослого в единице продукта (g/ml/pcs). Ужин и обед должны давать 500–800 ккал на порцию, завтрак 350–550, перекус 150–300 — сервер вернёт kcal в ответе; если мало или много, поправь количества и отправь снова.
4. POST /api/admin/recipes/batch с массивом до 50 рецептов. В ответе построчно {id, ok, kcal, error}. Ошибки исправь и отправь только неудавшиеся.
5. В конце дай мне таблицу: id, название, ккал, статус.

Поле image не заполняй. Тексты — на русском, переводы сделает сервер.

Вот блюда, которые нужно добавить:
- …`;

const ENDPOINTS: [string, string, string][] = [
  ["GET", "/api/admin/recipes/schema", "приёмы пищи, техника, теги, единицы, правила полей"],
  ["GET", "/api/admin/ingredients?q=", "база продуктов: id, название ru/en, единица, категория, КБЖУ на 100 г"],
  ["GET", "/api/admin/recipes?q=", "поиск по каталогу, до 40"],
  ["GET", "/api/admin/recipes/{id}", "рецепт целиком"],
  ["POST", "/api/admin/recipes", "один рецепт: создать (свой id или пустой) или обновить"],
  ["POST", "/api/admin/recipes/batch", "массив до 50; ответ {ok, failed, items:[{id, ok, kcal, error}]}"],
  ["DELETE", "/api/admin/recipes/{id}", "удалить"],
  ["GET / POST / DELETE", "/api/admin/collections[/{id}]", "редакционные подборки: name, slug, description, public, recipes[], names{}, descriptions{}"],
  ["GET / POST / DELETE", "/api/admin/partners[/{code}]", "партнёрские магазины"],
];

const EXAMPLE = `{
  "id": "chicken_thigh_honey_mustard",
  "title": "Куриные бёдра в медово-горчичном соусе",
  "description": "Бёдра запекаются в соусе из мёда, дижонской горчицы и чеснока.",
  "slot": "dinner", "timeMin": 45, "batch": true,
  "equipment": ["oven"], "tags": ["meat", "protein"],
  "steps": ["Духовку разогреть до 200 °C.", "Смешать мёд, горчицу, чеснок, соль и перец; обмазать бёдра.", "Запекать 35–40 минут до 74 °C внутри."],
  "ingredients": [{ "ingredientId": "chicken_thigh", "amount": 220 }, { "ingredientId": "honey", "amount": 10 }, { "ingredientId": "rice", "amount": 70 }],
  "image": ""
}`;

export function AdminApi({ onToast }: { onToast: (s: string) => void }) {
  const { t, lang } = useT();
  const confirm = useConfirm();
  const [keys, setKeys] = useState<ApiKey[] | null>(null);
  const [name, setName] = useState("");
  const [fresh, setFresh] = useState<ApiKey | null>(null);
  const load = () => api.apiKeys().then(setKeys).catch((e: Error) => onToast(e.message));
  useEffect(() => {
    void load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const create = async (e: FormEvent) => {
    e.preventDefault();
    try {
      setFresh(await api.apiKeyCreate(name.trim() || "Claude"));
      setName("");
      void load();
    } catch (err) {
      onToast((err as Error).message);
    }
  };
  const copy = async (s: string, msg: string) => {
    try {
      await navigator.clipboard.writeText(s);
      onToast(msg);
    } catch {
      onToast(t("cart.copy.fail"));
    }
  };
  const promptKey = fresh?.key ?? "rk_ВСТАВЬ_КЛЮЧ";
  return (
    <section className="admin__section apidoc">
      <div className="admin__panel">
        <h2 className="admin__h2">{t("admin.keys")}</h2>
        <p className="admin__hint">{t("admin.keys.hint2")}</p>
        {fresh && (
          <div className="keybox" role="status">
            <b>{fresh.name}</b>
            <code className="keybox__key">{fresh.key}</code>
            <button type="button" className="btn btn-primary btn-sm" onClick={() => copy(fresh.key ?? "", t("admin.keys.copied"))}>
              <Copy size={14} aria-hidden /> {t("admin.keys.copy")}
            </button>
            <small>{t("admin.keys.once")}</small>
          </div>
        )}
        {keys?.map((k) => (
          <div className="keyrow" key={k.id}>
            <KeyRound size={16} aria-hidden />
            <span className="keyrow__main">
              <b>{k.name}</b> <code>{k.prefix}</code>
              <small>{k.lastUsedAt ? t("admin.keys.used", { date: dateShort(k.lastUsedAt.slice(0, 10), lang) }) : t("admin.keys.unused")}</small>
            </span>
            <button
              type="button"
              className="planrow__del"
              aria-label={t("delete")}
              onClick={async () => {
                if (!(await confirm({ title: `${t("admin.keys.revoke")}: ${k.name}?`, ok: t("delete"), danger: true }))) return;
                await api.apiKeyDelete(k.id).catch((e: Error) => onToast(e.message));
                void load();
              }}
            >
              <Trash2 size={16} aria-hidden />
            </button>
          </div>
        ))}
        <form className="keyform" onSubmit={create}>
          <input className="form-control" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("admin.keys.name")} maxLength={40} />
          <button type="submit" className="btn btn-soft">
            <Plus size={16} aria-hidden /> {t("admin.keys.create")}
          </button>
        </form>
      </div>

      <div className="admin__panel">
        <div className="admin__row">
          <h2 className="admin__h2">Промпт для помощника</h2>
          <button type="button" className="btn btn-soft btn-sm" onClick={() => copy(PROMPT(promptKey), "Промпт скопирован")}>
            <Copy size={14} aria-hidden /> Скопировать промпт{fresh ? " с ключом" : ""}
          </button>
        </div>
        <p className="admin__hint">Открой Claude, Codex или ChatGPT, вставь промпт и допиши список блюд. {fresh ? "Ключ уже подставлен." : "Создай ключ выше — он подставится в промпт сам."}</p>
        <pre className="apidoc__pre">{PROMPT(promptKey)}</pre>
      </div>

      <div className="admin__panel">
        <h2 className="admin__h2">Ручки</h2>
        <p className="admin__hint">Все запросы — с заголовком <code>Authorization: Bearer rk_…</code>, JSON туда и обратно. Ключ действует с правами владельца.</p>
        <table className="apidoc__table">
          <tbody>
            {ENDPOINTS.map(([m, p, d]) => (
              <tr key={p}>
                <td><code>{m}</code></td>
                <td><code>{p}</code></td>
                <td>{d}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="admin__panel">
        <h2 className="admin__h2">Рецепт</h2>
        <p className="admin__hint">Название 2–80 знаков, подводка до 300, время 1–600 минут, 1–20 шагов до 500 знаков, 1–30 продуктов с amount на одну порцию. Ккал считает сервер — гарнир входит в состав, иначе ужин выйдет на 300 ккал.</p>
        <pre className="apidoc__pre">{EXAMPLE}</pre>
        <p className="admin__hint">Фото у новых рецептов нет — генерируем пакетом из репозитория ops. Переводы: <code>go run ./cmd/racionai recipes -to en,de,… -only id1,id2</code>. Добавленное через API живёт в базе сервера и деплоем не перетирается.</p>
      </div>
    </section>
  );
}
