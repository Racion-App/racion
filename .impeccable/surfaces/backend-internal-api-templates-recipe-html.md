---
version: 1
slug: "backend-internal-api-templates-recipe-html"
primary_target: "backend/internal/api/templates/recipe.html"
related_targets: ["backend/internal/api/templates/recipes.html","backend/internal/api/templates/layout.html"]
---

# Surface: страница рецепта и каталог (backend/internal/api/templates/recipe.html, recipes.html)

Scope: SSR-страницы `/recipe/{id}` и `/recipes`. Visitor mode: **Read** (посадочная из поиска: понять блюдо, решить готовить, забрать список).

Audience/job: человек из поиска «салат бигмак рецепт» или из плана. Действие: прочитать продукты и шаги, переключить порции, уйти собирать неделю. Content: 334 рецепта с граммовками, КБЖУ, ценой порции по Росстату, описанием; фото нет.

Constraints: Rubby DS binding; тот же CSS, что у приложения (`/assets/app.css`); без React; без эмодзи; одна гарнитура Inter, цифры — JetBrains Mono tabular.

## Direction contract

THESIS: Один белый лист с боковой панелью фактов — страница товара, где товар это блюдо. Отказывается от «статьи с водой»: каждый блок — данные.

OWN-WORLD: Rubby: лист `--ds-bg-elevated` на `--ds-bg-base`, радиус `xl`, тень `md`; тинт приёма пищи (orange/green/blue/purple) только в шапке; синий — один CTA; зелёный — ₽. Крошки — строка 13px с шевронами lucide, без ссылок-ковров. Заголовок 32/700 -0.02em, шаги 17px с номером в кружке `--ds-fill-3`. Раскладка ≥1024: `minmax(0,1fr) 360px`, панель липкая.

STORY: «Вот блюдо, вот что купить, вот как готовить, вот сколько это стоит и что даёт — собери неделю».

FIRST VIEWPORT: Шапка с тинтом: крошки → H1 → подводка → четыре факта в ряд (время, ккал, ≈₽, белок) → чипы тегов. Ниже начинаются две колонки: продукты с ± порций слева, «Пищевая ценность» таблицей справа.

FORM: Лист с боковой панелью — индекс 1 моего списка, выбран пользователем на раунде 241cb488. Сохранённые дисциплины: в тексте шага продукты помечены номерами строк списка (патентный чертёж); одна гарнитура (ежегодник); количества в табличном моно (экспозиция); шаги строгим ритмом (монтажный стол). Signature interaction: переключатель порций пересчитывает граммовки в строке и подсвечивает изменённые числа.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.
