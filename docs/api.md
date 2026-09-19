# API для добавления рецептов и подборок

Для тех, кто наполняет базу: руками или через помощника (Claude, Codex, ChatGPT). Ключ создаётся в админке → Обзор → «Ключи API»; он даёт те же права, что у твоего аккаунта, и показывается один раз.

Все запросы — с заголовком `Authorization: Bearer rk_…`, тело и ответы — JSON. База: `https://racion.app`.

## Готовый промпт для помощника

Скопируй целиком, подставь ключ и список блюд:

```
Ты добавляешь рецепты в базу сервиса Racion через его API.
Базовый адрес: https://racion.app
Заголовок для всех запросов: Authorization: Bearer rk_ВСТАВЬ_КЛЮЧ

Порядок работы:
1. GET /api/admin/recipes/schema — прочитай допустимые slot, equipment, tags и правила.
2. GET /api/admin/ingredients?q=слово — для каждого продукта найди id в базе (поиск по русскому, английскому названию или id). Продуктов, которых нет в базе, в рецепт не ставить: подбери ближайший из базы или пропусти рецепт и скажи мне, чего не хватает.
3. Собери рецепты по схеме: id (латиница и _), title, description, slot, timeMin, batch, equipment[], tags[] (только из схемы), steps[] (повелительное наклонение, с температурой и временами), ingredients[] = [{ingredientId, amount}] — amount на ОДНУ порцию взрослого в единице продукта (g/ml/pcs). Ужин или обед должны давать 500–800 ккал на порцию, завтрак 350–550, перекус 150–300 — сервер вернёт kcal в ответе, если мало или много, поправь количества и отправь снова.
4. POST /api/admin/recipes/batch с массивом до 50 рецептов. В ответе построчно {id, ok, kcal, error}. Ошибки исправь и отправь только неудавшиеся.
5. В конце дай мне таблицу: id, название, ккал, статус.

Поле image не заполняй. Тексты — на русском, переводы сделает сервер.

Вот блюда, которые нужно добавить:
- …
- …
```

## Ручки

| Метод | Путь | Что |
|---|---|---|
| GET | `/api/admin/recipes/schema` | приёмы пищи, техника, теги, единицы, правила полей |
| GET | `/api/admin/ingredients?q=` | база продуктов: id, название (ru/en), единица, категория, КБЖУ на 100 г/мл, pantry |
| GET | `/api/admin/recipes?q=` | поиск по каталогу (до 40) |
| GET | `/api/admin/recipes/{id}` | рецепт целиком |
| POST | `/api/admin/recipes` | один рецепт (создать или обновить по id) |
| POST | `/api/admin/recipes/batch` | массив рецептов, до 50; ответ `{ok, failed, items:[{id, ok, kcal, error}]}` |
| DELETE | `/api/admin/recipes/{id}` | удалить (свои рецепты каталога; seed-рецепты помечаются удалёнными) |
| GET | `/api/admin/collections` | редакционные подборки |
| POST | `/api/admin/collections` | создать или обновить подборку |
| DELETE | `/api/admin/collections/{id}` | удалить |
| GET / POST / DELETE | `/api/admin/partners`, `/api/admin/partners/{code}` | партнёрские магазины |
| GET / POST / DELETE | `/api/me/keys`, `/api/me/keys/{id}` | ключи (только с сессией из браузера или другим ключом) |

Лимит записи — тот же, что у админки: 20 запросов в секунду на адрес; пакет из 50 рецептов — один запрос.

## Рецепт

```json
{
  "id": "chicken_thigh_honey_mustard",
  "title": "Куриные бёдра в медово-горчичном соусе",
  "description": "Бёдра запекаются в соусе из мёда, дижонской горчицы и чеснока. Сочно внутри, карамель снаружи.",
  "slot": "dinner",
  "timeMin": 45,
  "batch": true,
  "equipment": ["oven"],
  "tags": ["meat", "protein"],
  "steps": [
    "Духовку разогреть до 200 °C.",
    "Смешать мёд, горчицу, чеснок, соль и перец; обмазать бёдра.",
    "Выложить в форму кожей вверх, запекать 35–40 минут до 74 °C внутри."
  ],
  "ingredients": [
    { "ingredientId": "chicken_thigh", "amount": 220 },
    { "ingredientId": "honey", "amount": 10 },
    { "ingredientId": "mustard_dijon", "amount": 8 },
    { "ingredientId": "garlic", "amount": 4 },
    { "ingredientId": "salt", "amount": 2 },
    { "ingredientId": "black_pepper", "amount": 0.5 },
    { "ingredientId": "rice", "amount": 70 }
  ],
  "image": ""
}
```

Правила проверки (ошибка приходит текстом в `error`): название 2–80 знаков, подводка до 300, время 1–600 минут, 1–20 шагов до 500 знаков, 1–30 продуктов с `amount > 0`, продукт обязан существовать в базе. Ккал считает сервер по продуктам — гарнир входит в состав (рис, картофель, гречка), иначе ужин выйдет на 300 ккал и планировщик будет его сторониться.

## Подборка

```json
{
  "id": "",
  "name": "Ужины за 20 минут",
  "slug": "dinners-20",
  "description": "Когда сил нет, а есть хочется: сковорода, одна кастрюля, без духовки.",
  "cover": "",
  "public": true,
  "recipes": ["chicken_thigh_honey_mustard", "shrimp_garlic_pasta"],
  "names": { "en": "20-minute dinners", "de": "Abendessen in 20 Minuten" },
  "descriptions": { "en": "When you're out of energy: one pan, one pot, no oven." }
}
```

`id` пустой — создать; с id — обновить. `cover` пустой — обложкой станет фото первого рецепта. `slug` — латиница, цифры, дефис; он в адресе `/collection/{slug}`.

## Примеры curl

```bash
export RACION_KEY=rk_…
curl -s https://racion.app/api/admin/recipes/schema -H "Authorization: Bearer $RACION_KEY" | jq .tags
curl -s "https://racion.app/api/admin/ingredients?q=курин" -H "Authorization: Bearer $RACION_KEY" | jq '.items[] | {id, name, unit}'
curl -s https://racion.app/api/admin/recipes/batch -H "Authorization: Bearer $RACION_KEY" -H "Content-Type: application/json" -d @recipes.json | jq .
```

## Что дальше с добавленным

- Фото: у новых рецептов его нет; генерируем пакетом (`scripts/gen_images_ima2.py --only id1,id2` в репозитории ops) и заливаем в `media`.
- Переводы на 14 языков: `cd backend && go run ./cmd/racionai recipes -to en,de,… -only id1,id2`.
- Рецепты, добавленные через API на проде, живут в базе сервера и не перетираются деплоем. Чтобы они попали и в seed (репозиторий `data`), выгрузи их: `GET /api/admin/recipes/{id}` и добавь в нужный `recipes_*.json`.
