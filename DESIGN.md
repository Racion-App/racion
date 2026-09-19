---
name: Рацион
description: Квиз из шести шагов, один непрерывный чек — меню недели и список покупок на одном листе, и страницы рецептов как лист с боковой панелью.
colors:
  bg-base: "#F2F2F7"
  bg-elevated: "#FFFFFF"
  bg-overlay: "rgba(255, 255, 255, 0.72)"
  label-primary: "rgba(0, 0, 0, 0.88)"
  label-secondary: "rgba(60, 60, 67, 0.65)"
  label-tertiary: "rgba(60, 60, 67, 0.42)"
  label-quaternary: "rgba(60, 60, 67, 0.20)"
  separator: "rgba(60, 60, 67, 0.18)"
  separator-opaque: "#D1D1D6"
  border-input: "rgba(60, 60, 67, 0.24)"
  fill-1: "rgba(120, 120, 128, 0.20)"
  fill-2: "rgba(120, 120, 128, 0.16)"
  fill-3: "rgba(118, 118, 128, 0.12)"
  fill-4: "rgba(116, 116, 128, 0.08)"
  blue: "#007AFF"
  blue-tint: "rgba(0, 122, 255, 0.10)"
  green: "#34C759"
  green-tint: "rgba(52, 199, 89, 0.10)"
  orange: "#FF9500"
  orange-tint: "rgba(255, 149, 0, 0.12)"
  purple: "#AF52DE"
  purple-tint: "rgba(175, 82, 222, 0.10)"
  red: "#FF3B30"
  red-tint: "rgba(255, 59, 48, 0.10)"
typography:
  page-title:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "30px"
    fontWeight: 700
    lineHeight: 1.12
    letterSpacing: "-0.022em"
  display:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "28px"
    fontWeight: 700
    lineHeight: 1.16
    letterSpacing: "-0.02em"
  headline:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "24px"
    fontWeight: 700
    lineHeight: 1.15
    letterSpacing: "-0.02em"
  title:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "20px"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.016em"
  step:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "17px"
    fontWeight: 400
    lineHeight: 1.5
  row-title:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "16px"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.01em"
  body:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "15px"
    fontWeight: 500
    lineHeight: 1.3
  caption:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.35
  label:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', sans-serif"
    fontSize: "12px"
    fontWeight: 600
    letterSpacing: "0.06em"
    lineHeight: 1.3
  num:
    fontFamily: "'JetBrains Mono', ui-monospace, 'SF Mono', Menlo, Consolas, monospace"
    fontSize: "14px"
    fontWeight: 600
    letterSpacing: "-0.01em"
    fontFeature: "'tnum' 1, 'lnum' 1, 'zero' 0"
rounded:
  xs: "4px"
  sm: "6px"
  md: "10px"
  lg: "14px"
  xl: "18px"
  2xl: "24px"
  pill: "9999px"
spacing:
  1: "4px"
  2: "8px"
  3: "12px"
  4: "16px"
  5: "20px"
  6: "24px"
  8: "32px"
  10: "40px"
  12: "48px"
  16: "64px"
components:
  button-primary:
    backgroundColor: "{colors.blue}"
    textColor: "{colors.bg-elevated}"
    rounded: "{rounded.md}"
    padding: "10px 16px"
    height: "48px"
  button-secondary:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.md}"
    padding: "10px 16px"
    height: "48px"
  button-secondary-hover:
    backgroundColor: "{colors.blue}"
    textColor: "{colors.bg-elevated}"
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.md}"
    padding: "0"
    size: "48px"
  button-ghost-hover:
    backgroundColor: "{colors.fill-3}"
  icon-button:
    backgroundColor: "transparent"
    textColor: "{colors.label-secondary}"
    rounded: "{rounded.md}"
    padding: "0"
    size: "36px"
  icon-button-hover:
    backgroundColor: "{colors.fill-3}"
    textColor: "{colors.label-primary}"
  tile:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.lg}"
    padding: "12px"
    height: "88px"
  tile-selected:
    backgroundColor: "{colors.blue-tint}"
    textColor: "{colors.label-primary}"
  chip:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.pill}"
    padding: "0 14px"
    height: "44px"
  chip-selected:
    backgroundColor: "{colors.blue-tint}"
    textColor: "{colors.blue}"
  chip-sm:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.pill}"
    padding: "0 12px"
    height: "34px"
  option:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.lg}"
    padding: "14px 16px"
    height: "64px"
  option-selected:
    backgroundColor: "{colors.blue-tint}"
  counter-button:
    backgroundColor: "{colors.fill-3}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.md}"
    size: "44px"
  portions-button:
    backgroundColor: "{colors.fill-3}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.md}"
    size: "36px"
  input:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.md}"
    padding: "0 14px 0 40px"
    height: "48px"
  receipt:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.xl}"
    padding: "0"
  recipe-sheet:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.xl}"
    padding: "0"
  fact-tile:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.md}"
    padding: "10px 12px"
  recipe-card:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.lg}"
    padding: "12px 14px 14px"
  panel:
    backgroundColor: "{colors.fill-4}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.lg}"
    padding: "14px 16px"
  panel-cta:
    backgroundColor: "{colors.blue-tint}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.lg}"
    padding: "14px 16px"
  step-number:
    backgroundColor: "{colors.fill-3}"
    textColor: "{colors.label-secondary}"
    rounded: "{rounded.pill}"
    size: "30px"
  filters:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.lg}"
    padding: "0 16px"
  auth-card:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.xl}"
    padding: "28px"
    width: "440px"
  account-card:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.lg}"
    padding: "16px"
  actionbar:
    backgroundColor: "{colors.bg-overlay}"
    padding: "10px 16px"
    height: "64px"
  bottom-sheet:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.label-primary}"
    rounded: "{rounded.2xl}"
    padding: "8px 20px 24px"
  toast:
    backgroundColor: "{colors.label-primary}"
    textColor: "{colors.bg-elevated}"
    rounded: "{rounded.pill}"
    padding: "10px 16px"
---

# Design System: Рацион

## Overview

**Creative North Star: «Чек» (seed cd59ffcb)**

Визуальный мир **унаследован** от Rubby Design System (Apple HIG поверх Bootstrap 5.3), завендоренной в `frontend/src/styles/design-system/` (`apple-tokens.scss`, `apple-typography.scss`, `apple-components.scss`, `apple-utilities.scss`, `README.md`). Все токены `--ds-*` (цвета в светлой и тёмной теме, радиусы, тени, шаг 4px, шрифтовые стеки, пружинная анимация) — источник правды; этот файл их не переопределяет, а описывает **слой поверхности**: как система легла на продукт «квиз → чек → рецепт» и какие локальные правила он к ней добавил. Локальные токены поверхности живут в `app.scss`: `--r-sheet-max: 560px`, `--r-gutter: 16px`, `--r-bar-h: 64px`, `--r-tabular`, `--r-safe-bottom`; страница рецепта добавляет один вычисляемый `--rp-tint` (тинт приёма пищи, берётся из `--ds-*-tint`).

Продукт — не дашборд, а один непрерывный документ. Результат квиза печатается как чек: заголовок недели, дни с блюдами, пунктирная линия отрыва, список покупок, итог с двойной линией. Страница рецепта (SSR, Go-шаблоны, тот же `app.css`) — тот же лист, но устроенный как карточка товара: шапка с фактами, слева продукты и шаги, справа панель фактов, внизу похожие. Плотность высокая, но воздух держится на разделителях, а не на отступах: строки блюд, позиций, продуктов и шагов отделены волосяными линиями `--ds-fill-4`, дни и блоки листа — `--ds-separator`. Единственный «бумажный» приём — линия отрыва с двумя вырезами по краям; никакой другой скевоморфной бумаги, зубчиков или текстур нет.

Цвет сдержан до предела: серо-белые нейтрали Apple, синий только на одном главном действии и текущем выборе, зелёный только на деньгах. Четыре тинта приёма пищи (завтрак/обед/ужин/перекус) существуют исключительно как фон-подложка — градиент шапки рецепта и полоска 4px на карточке; текстом или кнопкой они не бывают. Числа — моноширинные табличные (JetBrains Mono) в правой колонке, названия ведут по весу. Иконки — только `lucide-react` в React и те же lucide-контуры инлайн-SVG в шаблонах; эмодзи запрещены (правило продукта).

**Key Characteristics:**
- Rubby DS как источник правды; поверхность добавляет только `--r-*`, `--rp-tint` и классы чека/квиза/рецепта.
- Одна возвышенная белая карточка (`--ds-bg-elevated`) на сером грунте (`--ds-bg-base`): чек, лист рецепта, карточка входа.
- Синий = одно действие + выбор, зелёный = деньги, тинт слота = только подложка, красный = только ошибка/удаление. Больше цветов нет.
- Inter для текста, JetBrains Mono `tnum` для всех чисел через класс `.num` — и в React, и в Go-шаблонах.
- Пружинная анимация `--ds-ease-spring`; единственный авторский момент — «печать» чека; на рецепте только `reprint` пересчитанных граммовок.
- Каждая оценка в рублях несёт «≈»; цифра без «≈» — только целевой бюджет пользователя.
- Один и тот же топбар в React и в SSR: бренд · «Рецепты» · кабинет · тема.

## Colors

Палитра — системные цвета Apple из `apple-tokens.scss`; поверхность использует семь ролей из двенадцати и не вводит ни одного собственного цвета.

### Primary
- **Системный синий** (`--ds-blue`, `{colors.blue}`): только главная кнопка `.btn-primary` («Дальше», «В магазин», «Собрать корзину» в режиме магазина, «Собрать неделю» на рецепте и в кабинете, «Войти»), выбранная плитка/чип/опция (рамка + `--ds-blue-tint` фон + inset-кольцо 1px), заполненные сегменты прогресса квиза, иконка бренда в топбаре, иконка кабинета у вошедшего (`.theme-btn.is-user`), счётчик активных фильтров `.filters__count`, hover названий (блюдо, карточка рецепта, крошка, ссылка-ссылка на продукт в шаге), `caret-color`, `accent-color`, `::selection`.
- **Синяя подложка** (`--ds-blue-tint`): фон выбранного состояния, вспышка `reprint` у заменённой строки, панель `.panel--cta` вокруг кнопки «Собрать неделю», аватар кабинета `.account__avatar`.

### Secondary
- **Системный зелёный** (`--ds-green`): итоговая сумма продуктов в шапке чека и в `.totals__row--main .num`; залитый кружок `.item__check[aria-checked="true"]`; на рецепте — факт «Продукты ≈ ₽» (`.rpage__facts .is-rub`), цена порции `.panel__big`, `≈ ₽` в мета-строке карточки (`.rcard__meta .is-rub`). Никогда не на кнопках.
- **Тинты приёма пищи** (`--ds-orange-tint` завтрак, `--ds-green-tint` обед, `--ds-blue-tint` ужин, `--ds-purple-tint` перекус): только `--rp-tint` — верх градиента шапки `.rpage__hero`, уходящего в `--ds-bg-elevated`. Плотные `--ds-orange` / `--ds-green` / `--ds-blue` / `--ds-purple` — только полоска `.rcard__cover` высотой 4px у карточки без фото.

### Tertiary
- **Системный красный** (`--ds-red` + `--ds-red-tint`): `.error-inline` (ошибка загрузки, предупреждение о бюджете, ошибка входа) и hover кнопок удаления (`.planrow__del`, `.item__del`, `.dish__swap--no`).
- **Системный оранжевый как текст** (`--ds-orange`): единственное место — факт бюджета `.totals__row .is-over`, когда факт дороже цели более чем на 5%.

### Neutral
- **Грунт** (`--ds-bg-base`): фон страницы и вырезы линии отрыва.
- **Лист** (`--ds-bg-elevated`): чек, лист рецепта, карточка входа, карточки рецептов, плитки фактов, чипы, опции, счётчик, подсказки, нижний лист, блок фильтров, карточка профиля.
- **Стекло** (`--ds-bg-overlay` + `--ds-blur-md`): только `.actionbar`.
- **Текст** (`--ds-label-primary` → `secondary` → `tertiary` → `quaternary`): название / метаданные, подводка, количество, крошки-ссылки / «почему выбрано», слот, номер строки продукта, подпись факта, надстрочный номер в шаге, категория списка / зачёркнутые цены купленного, шеврон крошек, маркеры «Коротко».
- **Разделители**: `--ds-separator` (граница карточек и листов, дни, шапка чека, шапка рецепта, правая граница `.rpage__main`, верх `.rpage__foot`), `--ds-separator-opaque` (пунктир отрыва, кружок чекбокса, `.btn-secondary`), `--ds-fill-4` (волосяные линии между блюдами, позициями, продуктами `.ings__row`, шагами `.steps__item`, группами фильтров), `--ds-fill-3` (линии таблицы `.nutri` и списка `.rlist`).
- **Заливки** (`--ds-fill-1..4`): грип нижнего листа и скроллбар (`fill-1`), hover (`fill-2`), кнопки счётчика / порций / закрытия / номера шагов / сегмент-контрол (`fill-3`), панели боковой колонки `.panel`, детские строки `.kidrows`, skeleton-градиент (`fill-4`→`fill-3`).

### Named Rules
**The Restrained Palette Rule.** На любом экране цвет несут максимум четыре вещи: одна синяя кнопка, синие выбранные элементы, зелёные деньги и тинт слота как подложка шапки/полоска карточки. Всё остальное — нейтрали `--ds-label-*` и `--ds-fill-*`. Новый экран, которому «нужен» оранжевый или фиолетовый текст, нарушает мир.

**The Green Is Money Rule.** Зелёный появляется только у сумм в рублях (`≈ N ₽` в шапке чека, «Итого», факт «Продукты», цена порции, `≈ ₽` на карточке) и у отметки «куплено». Не для успеха форм, не для кнопок, не для «в бюджете». Каждый `≈ ₽` вне чека несёт класс `.is-rub`.

**The One Blue CTA Rule.** На странице ровно один `.btn-primary`. На рецепте это «Собрать неделю» в `.panel--cta`; топбар, шапка и подвал держат только текстовые ссылки. Второе синее действие на экране — дефект, а не акцент.

**The Slot Tint Is Wash Rule.** Тинт приёма пищи — только фон: градиент `--rp-tint → --ds-bg-elevated` в шапке рецепта и полоска 4px на карточке. Никогда текст, рамка, иконка или кнопка.

**The Token-Only Rule.** Поверхность не пишет hex: каждый цвет — `var(--ds-*)`. Единственные литералы — `#fff` на галочке купленного и на счётчике фильтров, `rgba(0,0,0,0.38)` у `::backdrop` нижнего листа и `#fff` фона в `@media print`.

## Typography

**Display Font:** Inter (переменный, 100–900, самохостинг `/fonts/InterVariable.woff2`, `preload` в обоих каркасах, полная кириллица, «₽» и «≈»)
**Body Font:** Inter (тот же файл)
**Label/Mono Font:** JetBrains Mono 500 / 600 / 700 (самохостинг `/fonts/JetBrainsMono-{Medium,SemiBold,Bold}.woff2`), стек `--ds-font-mono`

**Character:** Inter с отрицательным трекингом на заголовках (−0.02em) даёт плотный, «системный» голос Apple; JetBrains Mono с `tnum`/`lnum` выстраивает рубли, граммы и калории в ровные столбики, как на кассовом чеке. Два шрифта — два вида информации: слова и числа. Второй гарнитуры для заголовков нет и не будет.

### Hierarchy
- **Page title** (700, 30px → 36px на ≥1024, 1.12, −0.022em): заголовок рецепта `.rpage__title`; `max-width: 24ch`, `text-wrap: balance`.
- **Display** (700, 28px, 1.16, −0.02em): вопрос квиза `.quiz__title`, заголовок каталога `.pages__title`, заголовок входа `.auth__title`.
- **Headline** (700, 24px, 1.15, −0.02em): заголовок чека `.receipt__title`, заголовок кабинета `.account__title`. 22px/−0.018em — заголовок рецепта в нижнем листе. 26px/600 — цена порции `.panel__big` (в `.num`).
- **Title** (700, 20px, −0.016em): «Список покупок», «Итого», «Продукты» / «Как готовить» / «Коротко» / «Похожие» `.rpage__section h2`, значение счётчика (600).
- **Step** (400, 17px, 1.5): текст шага `.steps__item p`; факт в плитке `.rpage__facts dd` — 17/600 −0.01em.
- **Row title** (600, 16px, 1.3, −0.01em): название блюда `.dish__title`, заголовок опции, подпись счётчика, строка продукта `.ings__row` (400 у названия, 500 у количества), подводка `.rpage__lead` (400, 1.45, `max-width: 62ch`); 17px — бренд в топбаре и позиция списка в режиме магазина.
- **Body** (500–600, 15px, 1.3): плитки, чипы, позиции списка, ингредиенты, подсказка квиза (400), заголовок карточки `.rcard__title` (600), сводка фильтров (600), «Коротко» `.facts` (400, 1.45).
- **Caption** (400–500, 13–14px, 1.35): метаданные чека, суммы дня, количество, цена позиции, кнопка отрыва (600), `topbar__note`, крошки (500 у ссылок), таблица `.nutri` (14), заголовок панели `.panel__title` (13/600 +0.02em), «Рецепты» в топбаре (14/600), группа фильтров (13/600); 12.5px — `.rcard__meta`, `.rpage__note`, `.dish__why`.
- **Label** (600, 12px, +0.06em, uppercase): категория списка `.list__cat` и `h3` в нижнем листе. 11–12px без капса: подпись факта `.rpage__facts dt` (11/600 `tertiary`), номер строки продукта `.ings__n` (12), примечание к упаковке `.ings__name small` (12), подпись «за порцию» (12/500), слот приёма пищи, примечание итога.
- **Num** (`.num`: `--ds-font-mono`, `tnum lnum`, `zero 0`, −0.01em): любое число — рубли, ккал, минуты, граммы, КБЖУ, проценты, номер строки, надстрочный номер продукта в шаге (`sup` 11/600). Размер наследует от контекста (11–26px), вес 600 для главных сумм.

### Named Rules
**The Mono Column Rule.** Число никогда не набирается Inter. Всё, что считается (`₽`, ккал, мин, г, мл, шт, %, Б/Ж/У, номера строк), оборачивается в `.num` и выравнивается по правому краю колонки (`.dish__nums`, `.day__sum`, `.item__cost`, `.totals__row .num`, `.ings__qty`, `.nutri td`). Исключение — «по вкусу» у продукта без граммовки: это слово, не число, и `.num` не получает.

**The Honest Numbers Rule.** Любая оценка в рублях выводится через `approxRub()` / `rub` и несёт «≈ » перед суммой: цена блюда, сумма дня, категория, позиция, «Итого», факт «Продукты», цена порции, `≈ ₽` на карточке. Без «≈» пишется только целевой бюджет, который задал сам пользователь; примечание итога и панель «Стоимость» объясняют метод («средние цены Росстата», «в магазине будет иначе»).

**The Weight Leads Rule.** В строке ведёт вес, а не цвет: название блюда 600/`label-primary`, всё вокруг — 500/`label-secondary`–`tertiary`. На рецепте так же: заголовок раздела 700, продукт 400/`primary`, количество 500/`secondary`, номер 12/`tertiary`. Заголовок дня «ПН» — 700 с +0.02em, дата рядом — 500/`tertiary`.

**The Step-Ref Rule.** Продукт в тексте шага помечается прямо в слове: `<a class="steps__ref" href="#ing-N">слово<sup class="num">N</sup></a>` — точечное подчёркивание `--ds-label-quaternary`, номер надстрочно 11/600 моно `tertiary`, hover синий у слова и номера. Никаких чипов или списков продуктов под шагом: ссылка живёт в предложении, номер совпадает с `.ings__n` в списке продуктов.

**The Russian Plural Rule.** Счётчики склоняются функцией `plural(n, "рецепт", "рецепта", "рецептов")`; «294 рецептов» — дефект копирайта, а не стиль.

## Layout

Одна колонка по центру: `.shell` шириной `100%`, `max-width: var(--r-sheet-max)` (560px), поля `var(--r-gutter)` (16px). `#root` — flex-колонка на `100dvh`; страница скроллится обычно (поверхность возвращает `overflow` у `html/body`, которые DS запирает под app-shell). Топбар 56px.

**Топбар (один для React и SSR).** `.topbar` — `flex; space-between; gap 12px; height 56px`. Слева `.topbar__brand`: lucide `ReceiptText` 22px синим + «Рацион» 17/700 −0.014em. Справа `.topbar__right` (`gap: 6px`): в React — `topbar__note` 13/500 `secondary` (скрыта ≤480px) или переданный `right`; затем в обоих каркасах текстовая ссылка `.pages-nav__link` «Рецепты» (14/600 `secondary`, `.is-current` — `primary`) · иконка кабинета `.theme-btn` 36px (`UserRound` 18px; у вошедшего `.is-user` — синяя; гостя ведёт на `/login`) · переключатель темы `.theme-btn[data-theme-btn]` (36px, показывает одну из трёх иконок `.i-auto` / `.i-light` / `.i-dark`, цикл auto → light → dark → auto через `localStorage["racion.theme"]` и `data-theme` на `<html>`). Никаких кнопок `.btn` в топбаре; никаких вкладок.

Нижняя панель `.actionbar` фиксирована, высота учитывается токеном `--r-bar-h` (64px) + `--r-safe-bottom` (`env(safe-area-inset-bottom)`): и квиз, и чек добавляют `padding-bottom: calc(var(--r-bar-h) + var(--r-safe-bottom) + 16–24px)`, тост поднимается на ту же величину. Кнопки в панели растягиваются поровну (`flex: 1`, 48px), ghost-иконка — квадрат 48px. У SSR-страниц, входа и кабинета панели действий нет: действие живёт в контенте.

Ритм отступов — сетка 4px из DS: внутренние поля карточек 12–14px, шапка чека 20px, дни `14px 20px 10px`, строки блюд `9px 0`, продукты `9px 0`, шаги `12px 0`, позиции списка `7px 0` (min-height 40px), группы `10px 0 6px`. Между элементами квиза 8–10px (`.tiles`, `.chips`, `.options`, `.quiz__group`), между блоками шага 20px, между разделами рецепта 22px, между панелями боковой колонки 12px, между страницами каталога `.pages` 16px.

Сетка строки блюда: `minmax(0,1fr) auto auto` (текст / числа / замена 36px). Позиция списка: `auto minmax(0,1fr) auto` (чекбокс 22px / название + количество / цена). Строка продукта `.ings__row`: `26px minmax(0,1fr) auto` (номер / название + примечание к упаковке / количество), `align-items: baseline`. Шаг `.steps__item`: `32px minmax(0,1fr)` (кружок 30px / текст). Плитки магазинов: 3 колонки, на ≤360px — 2. Факты рецепта `.rpage__facts`: 2 колонки, с 560px — 4. Карточки `.rgrid`: `repeat(auto-fill, minmax(240px, 1fr))`, `gap 12px`; `.rgrid--tight` — 200px (похожие). Группа фильтров: `96px minmax(0,1fr)`, на ≤560px — одна колонка.

**Лист рецепта (`.rpage`).** Один белый лист `--ds-radius-xl` + `--ds-separator` + `--ds-shadow-md`, `overflow: clip`. Порядок сверху вниз: `.rpage__hero` (крошки → H1 → подводка → `<dl>` из четырёх фактов «Готовить / Порция / Продукты / Белок» → чипы тегов `.chip--sm`; `padding 20px 20px 22px`, низ `--ds-separator`, фон-градиент `--rp-tint → --ds-bg-elevated`) → `.rpage__cols` (на телефоне одна колонка: `.rpage__main` затем `.rpage__aside`) → `.rpage__foot` «Похожие» на всю ширину под обеими колонками, отделён `--ds-separator`. `.rpage__main` (`8px 20px 24px`, `gap 22px`): «Продукты» с переключателем порций `.portions` в шапке раздела, нумерованный `.ings`, примечание `.rpage__note` про кладовые; «Как готовить» `.steps`; «Коротко» `.facts` — только если есть факты, не повторяющие плитки шапки. `.rpage__aside` (`0 20px 24px`, `gap 12px`): панели `.panel` — «Пищевая ценность» с таблицей `.nutri` (название / значение / % от 2000 ккал), «Стоимость» с `.panel__big`, «Понадобится», `.panel--cta` с единственной синей кнопкой.

**Широкий экран (≥1024px):** `.shell--wide` растягивается до 1120px (чек, кабинет, все SSR-страницы). Чек становится сеткой `minmax(0,1fr) 400px`: шапка и предупреждение на всю ширину, неделя слева, список справа за вертикальным пунктиром `2px dashed --ds-separator-opaque`, липкая шапка списка, итог на всю ширину внизу. Линия отрыва скрыта — обе половины видны. Слот приёма пищи переезжает в узкую колонку 64px слева (`.dish__slot`). Панель действий прижимает кнопки вправо (`min-width: 200px`). Рецепт: `.rpage__cols` → `minmax(0,1fr) 360px`, шапка `28px 32px 26px`, H1 36px, `.rpage__main` `8px 32px 32px` с правой границей `--ds-separator`, `.rpage__aside` `20px 24px 32px`, `align-self: start`. Один документ без вложенного скролла.

**Липкая панель (условие).** `.rpage__aside` получает `position: sticky; top: 12px` только при `min-width: 1024px` **и** `min-height: 760px` — там, где панель целиком помещается в окно. На низких окнах она просто скроллится вместе с листом; вложенного `overflow: auto` у панели нет.

**Кабинет (≥900px):** `.account` — сетка `minmax(0,1fr) 320px`, `column-gap 32px`: шапка, сегмент-контрол и секции в первой колонке, профиль `.account__profile` во второй на `grid-row: 1 / span 3` без верхней линии. **Вход (≥640px):** `.auth__card` — карточка 440px по центру, `margin-top 24px`, `padding 28px`; на телефоне тот же контент без карточки.

**Режим магазина (`.is-store`):** тот же лист, но шапка, меню, дни, отрыв и предупреждение скрыты; список крупнее (позиции 52px, название 17px, чекбокс 28px, показывается `.item__used`). На десктопе чек схлопывается в одну колонку без пунктира.

## Elevation & Depth

Гибрид: тональные слои плюс многослойные тени Apple из DS. Грунт `--ds-bg-base` серый, лист белый — сама разница тонов уже даёт глубину, тени лишь подтверждают её. Три уровня: чек, лист рецепта, карточка входа и нижний лист — приподняты тенью; элементы управления, карточки рецептов и панели — почти плоские; `actionbar` — стекло над контентом.

### Shadow Vocabulary
- **Лист** (`--ds-shadow-md`: `0 1px 2px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.07)`): чек, `.rpage`, `.auth__card` на ≥640px — единственная приподнятая поверхность своей страницы; плюс граница `1px --ds-separator` и `overflow: clip`. Карточка рецепта `.rcard` поднимается до `--ds-shadow-md` и `translateY(-1px)` только на hover.
- **Плитка / карточка** (`--ds-shadow-xs`: `0 1px 2px rgba(0,0,0,0.04)`): плитки магазинов, `.rcard` в покое; чипы, опции, счётчик, плитки фактов, блок фильтров, `.account__card`, `.planrow` — без тени, только граница.
- **Панель внутри листа** — без тени и границы: `.panel` на `--ds-fill-4` читается как тональный слой внутри белого листа.
- **Сегмент** (`--ds-shadow-sm`): активная кнопка `.viewswitch`.
- **Нижний лист** (`--ds-shadow-xl`) и **тост** (`--ds-shadow-lg`): всплывающие поверхности.
- **Главная кнопка**: собственная синяя тень DS (`.btn-primary`), поверхность её не трогает.
- **Фокус** (`--ds-shadow-focus`: `0 0 0 4px rgba(0,122,255,0.22)`): кольцо на `:focus-visible` у каждого интерактива, `outline: none`.
- **Стекло**: `.actionbar` — `--ds-bg-overlay` + `backdrop-filter: var(--ds-blur-md)` + верхняя граница `--ds-separator`.
- **Print**: тень и граница чека снимаются, фон белый.

### Named Rules
**The One Sheet Rule.** На странице одна приподнятая поверхность — чек, лист рецепта или карточка входа. Внутри листа глубины нет: дни, строки, продукты, шаги и панели разделяются линиями и тональными подложками `--ds-fill-4`, а не карточками в карточке. Плитки фактов в шапке рецепта — белые на тинте с границей, без тени.

**The Ring Not Outline Rule.** Фокус — всегда `--ds-shadow-focus` через `box-shadow`, `outline: none`; выбранное состояние — `0 0 0 1px var(--ds-blue) inset` поверх синей рамки. Двух колец одновременно не бывает.

## Shapes

Радиусы только из шкалы DS. Лист чека, лист рецепта и карточка входа — `--ds-radius-xl` (18px), нижний лист — `--ds-radius-2xl` (24px) сверху, плитки/опции/счётчик/карточки рецептов/панели/блок фильтров/`.planrow`/`.account__card`/фото рецепта — `--ds-radius-lg` (14px), кнопки, поля, подсказки, плитки фактов, кнопки счётчика, порций, темы и замены, `.kidrows` — `--ds-radius-md` (10px), кнопка `.dish__main` и `.item__del` — 6px, вспышка `reprint` у количества — 4px. Чипы (44px и `.chip--sm` 34px), кнопка отрыва, тост, сегменты прогресса, грип, скроллбар, счётчик фильтров — пилюли (`--ds-radius-pill` / 999px). Круги: чекбокс позиции (22px, в магазине 28px), кнопка закрытия листа (36px), номера шагов (30px на рецепте, 26px в нижнем листе), аватар кабинета (48px), вырезы линии отрыва (20px).

Границы — 1px `--ds-separator` на всех поверхностях; чекбокс 1.5px `--ds-separator-opaque`. Волосяные линии `--ds-fill-4` между строками (блюда, позиции, продукты, шаги, группы фильтров), `--ds-fill-3` внутри таблиц и коротких списков (`.nutri`, `.rlist`), `2px solid --ds-label-primary` — черта над итогом (единственная толстая сплошная линия, как на кассовом чеке), `2px dashed --ds-separator-opaque` — линия отрыва и десктопный разделитель колонок чека; колонки рецепта разделяет обычная `1px --ds-separator`. Точечная линия `1px dotted --ds-label-quaternary` — только под ссылкой на продукт в шаге. Шеврон крошек — lucide `ChevronRight` 14px `quaternary`; шеврон `<details>` фильтров — CSS-уголок 8px с поворотом. Чек и лист рецепта обрезают содержимое (`overflow: clip`).

## Components

### Buttons
- **Shape:** скруглённый прямоугольник (`--ds-radius-md`, 10px), 600, `-0.008em`; в `.actionbar` — 48px высоты, 16px.
- **Primary (`.btn-primary`, из DS):** синий градиент на `--ds-blue`, белый текст, inset-блик и синяя тень; один на страницу — «Дальше» / «Готово» в квизе, «В магазин» на чеке, «Собрать корзину» в магазине, «Собрать неделю» на рецепте (`.btn-lg`, на всю ширину панели) и в пустом кабинете, «Войти» на входе.
- **Secondary (`.btn-secondary`):** белый с рамкой `--ds-separator-opaque`; hover заливается синим с белым текстом. «Назад» / «Дальше» в пагинации каталога, «Сохранить» имя в профиле.
- **Ghost (`.btn-ghost`):** прозрачный, hover `--ds-fill-3`; в панели — квадрат 48px под одну иконку. `.btn-sm` — «Изменить ответы» в топбаре чека; «Выйти» в профиле. `.btn-soft` — мягкая вторичная («Собрать ещё», «Сбросить» в пустом каталоге).
- **Icon button (`.theme-btn`):** 36px, прозрачный, `label-secondary`, hover `--ds-fill-3` + `primary`; используется для кабинета и темы в топбаре. `.is-user` — синий.
- **Hover / Active / Focus:** `translateY(-0.5px)` / `scale(0.97)` с `--ds-ease-spring` / `--ds-shadow-focus`. `:disabled` — opacity 0.45.

### Quiz controls
- **Tile (`.tile`, `role="radio"` в `role="radiogroup"`):** белая карточка 88px min, иконка lucide 22px сверху (`label-secondary`), название 15/600, подпись 12/500 `tertiary`. Выбрана (`aria-checked`/`.is-on`): синяя рамка, `--ds-blue-tint`, иконка синяя. `:active scale(0.97)`.
- **Chip (`.chip`, `aria-pressed`):** пилюля 44px, 15/500; выбран — синий текст 600, синяя рамка, tint, галочка lucide 16px внутри. `.chip--remove` — 36px на `--ds-fill-3` с крестиком. **`.chip--sm`** — 34px, `0 12px`, 13px: теги в шапке рецепта и опции фильтров каталога (ссылки `<a>`, `aria-pressed="true"` у активной; hover — синяя рамка и текст).
- **Counter (`.counter`):** карточка `--ds-radius-lg`, подпись 16/600 + `sub` 13, кнопки ± 44×44 на `--ds-fill-3`, значение 20/600 шириной 36px. **Portions (`.portions`, `role="group"`):** те же `.counter__btn`, но 36×36, `<output class="num">` 16/600 шириной 28px; диапазон 1–12, пересчёт `data-amount × n` в единицах `г/кг`, `мл/л`, `шт`, каждая изменённая ячейка получает `.is-fresh` (вспышка `reprint`).
- **Option (`.option`, `aria-checked`):** строка 64px min, заголовок 16/600 + подпись 13, значение справа `label-secondary`; выбрана — как плитка, значение синим 600.
- **Progress (`.quiz__progress`, `role="progressbar"`):** сегменты 4px пилюли на `--ds-fill-2`; пройденные — `--ds-blue`, текущий — `--ds-blue` с opacity 0.55.
- **Step enter:** `.quiz__step` въезжает `step-in` за `--ds-dur-base` по `--ds-ease-spring`.
- **Store logo (`StoreMark`):** официальный SVG сети из `public/logos/` высотой 24px в `.tile__logo` (28px); название сети только для читалок; выбор — плитка как обычно + `scale(1.04)` логотипа.
- **Equipment tile (`.tile--eq`):** плитка в ряд, иконка `EquipmentIcon` 26px слева, подпись 14; выбрана — иконка синяя и оживает одна деталь (`eq-steam`, `eq-glow`, `eq-spin`, `eq-wobble`); `prefers-reduced-motion` останавливает всё.
- **Kid card (`.kid`):** карточка как Counter; возраст — `<select>`; режим кормления — `.option--compact`; переключатель смеси `.switch` (трек 46×28, включён — `--ds-green`, единственное место, где зелёный не про деньги: DS-семантика «включено»).
- **Budget (`.segmented` + `.money`):** сегмент-контрол 40px на `--ds-fill-3`, под ним поле 64px с суммой 32/600; пресеты — чипы с суммой в `.num`.
- **Segmented control, выбранный сегмент (`[aria-checked=true]` / `[aria-selected=true]`):** плашка `bg-elevated` с тенью `sm` и кольцом 1px `separator`, текст синий (`--ds-blue`, 14/600); в тёмной теме плашка — `--ds-fill-1` без тени. Синий текст — главный признак выбора, чтобы вкладки читались и на тёмном.
- **Loading (`ReceiptLoader`):** в кнопке «Собрать рацион» — 22px SVG-чек, печатающий четыре строки штрихом (SMIL). Единственный лоадер поверхности.

### Inputs / Fields
- **Style:** Bootstrap `.form-control` под токенами DS: белый фон, рамка `--ds-border-input`, `--ds-radius-md`; в `.search` (квиз и `.pages__search` каталога, `max-width 480px`) — 48px, 16px, иконка lupa 18px слева (`padding-left: 40px`); поля входа и профиля `.auth__field` — подпись 14/600 `secondary` над полем 48px.
- **Focus:** синяя рамка + `--ds-shadow-focus`.
- **Suggest (`.suggest`, `role="listbox"`):** белые строки `--ds-radius-md`, hover `--ds-fill-4`.
- **Filters (`<details class="filters">`):** белый блок `--ds-radius-lg` с границей, `padding 0 16px`; `<summary>` 15/600 на `14px 0` с CSS-уголком справа (поворот на `[open]`), при активных фильтрах — счётчик-пилюля `.filters__count` 22px синяя с белой цифрой `.num` и ссылка «сбросить» 13/500; открыт по умолчанию, если есть активные фильтры. Группы `.filters__group` — `96px | 1fr` (подпись 13/600 `secondary` / чипы `.chip--sm`), между группами `--ds-fill-4`.

### Cards / Containers
- **Receipt (`.receipt`):** белый лист `--ds-radius-xl`, граница, `--ds-shadow-md`, `overflow: clip`. `aria-busy="true"` со skeleton-строками во время загрузки.
- **Recipe sheet (`.rpage`):** тот же лист; модификатор `.rpage--{breakfast|lunch|dinner|snack}` задаёт `--rp-tint`; анатомия описана в Layout.
- **Fact tile (`.rpage__facts > div`):** белая плитка `--ds-radius-md` с границей `--ds-separator`, `10px 12px`; `dt` 11/600 `tertiary`, `dd` 17/600 `.num`; факт «Продукты» — `.is-rub` зелёный; факт «Продукты» показывается только у оценённых рецептов.
- **Recipe card (`.rcard`, `<a>`):** `--ds-radius-lg`, белая, граница, `--ds-shadow-xs`, `grid-template-rows: auto 1fr`, `overflow: hidden`. Верх — либо фото `.rcard__img` 4:3 (`loading="lazy"`), либо **полоска `.rcard__cover` высотой 4px** цвета слота (`--ds-orange` завтрак / `--ds-green` обед / `--ds-blue` ужин / `--ds-purple` перекус). Тело `12px 14px 14px`, `gap 6px`: заголовок 15/600, мета-строка 12.5 `secondary` — слот · `N мин` · `N ккал` · `≈ N ₽` (последнее `.is-rub` зелёное), числа в `.num`. Hover: `--ds-shadow-md`, `translateY(-1px)`, заголовок синий. Без фото карточка компактная: никакой цветной «обложки» с текстом внутри.
- **Panel (`.panel`):** тональная подложка `--ds-fill-4`, `--ds-radius-lg`, `14px 16px`; `.panel__title` 13/600 +0.02em `secondary`, `.panel__sub` 13/1.45 `tertiary`, `.panel__big` 26/600 зелёный `.num` с подписью `small` 12/500 в Inter. `.panel--cta` — на `--ds-blue-tint`, кнопка на всю ширину + пояснение.
- **Nutrition table (`.nutri`):** 14px, три колонки — `th` 500 `secondary` слева, `td.num` справа, `td.nutri__pct` 44px `tertiary`; строки разделены `--ds-fill-3`, первая без линии. Строка «На 100 г» без процента.
- **Auth card (`.auth__card`):** на ≥640px белая карточка 440px, `--ds-radius-xl`, `28px`, `--ds-shadow-md`; заголовок 28/700, подводка 15 `secondary`, сегмент-контрол «Вход / Регистрация», поля, одна синяя кнопка, `.auth__foot` 14 `secondary`.
- **Account card (`.account__card`) / Plan row (`.planrow`):** белые `--ds-radius-lg` с границей, без тени; `.planrow` — `minmax(0,1fr) auto`, заголовок 15/600 (hover синий), мета 13 `secondary` с `.num`, справа `.planrow__del` 36px (hover `--ds-red-tint` + красный). `.account__avatar` — круг 48px на `--ds-blue-tint` с синей буквой 20/700.
- **Bottom sheet (`.bsheet`, `<dialog>`):** прижат к низу, `max-width: --r-sheet-max`, `--ds-radius-2xl` сверху, `--ds-shadow-xl`, грип 36×5, круглая кнопка закрытия 36px, backdrop 38% чёрного + blur 2px; открытие `sheet-up`.
- **Skeleton:** `--ds-radius-md`, градиент `fill-4 → fill-3 → fill-4`, shimmer 1.2s.
- **Toast (`role="status"`):** инвертированная пилюля, 14/500, над панелью действий.

### Navigation
- **TopBar (`.topbar`, React и SSR идентичны по структуре):** 56px; бренд `ReceiptText` 22px синим + «Рацион» 17/700; справа `.topbar__right` `gap 6px`: [нота 13/500 или переданный `right`] · «Рецепты» `.pages-nav__link` 14/600 · язык `.theme-btn--lang` (36px, иконка `Languages` 18px, в правом нижнем углу иконки флаг текущего языка `.theme-btn__flag` 12×9 радиус 2px с ободком 1.5px цвета кнопки, внутри кнопки; открывает лист выбора языка, см. LangDialog) · кабинет `.theme-btn` 36px · тема `.theme-btn[data-theme-btn]` 36px. Никаких вкладок и никаких `.btn` в шапке.
- **LangDialog (`.langlist`, React `LangDialog.tsx` и SSR `<dialog class="bsheet langdlg">` в `layout.html`):** нижний лист (`.bsheet`) с заголовком «Язык», подводкой (`lang.auto` если язык определён автоматически, иначе `lang.hint`) и списком `.langlist__item` 52px, gap 4px: флаг `.langlist__flag` 28×21 радиус 4px с внутренней обводкой `separator` (`flag-icons`), название на самом языке 16/600 + английское название 12.5 `label-tertiary` (и `· 87%` у неполного перевода), справа `Check` у текущего. Текущий пункт `.is-on` — `blue-tint` + рамка `blue`, hover — `fill-4`. Внизу `.langlist__foot` 13 `label-tertiary` — как добавить язык. Никаких выпадающих селектов и кодов языков в шапке.
- **Помощник в форме рецепта (`.ownform__ai`):** блок на подложке `fill-4` радиус md, две кнопки `.btn-soft.btn-sm` с иконкой `Sparkles` («Улучшить текст», «Перевести на …»), после ответа — `.btn-link` «Вернуть как было» с `Undo2`; под кнопками строка состояния 13 `label-tertiary` (подсказка → «Помощник думает…» синим при `aria-busy` → «Готово…»). Блок показывается только если `/api/meta` вернул `ai: true`.
- **Crumbs (`.crumbs`, `<nav aria-label="Путь">`):** 13px, ссылки 500 `secondary` (hover синий), между ними lucide-шеврон 14px `quaternary`; не больше двух звеньев («Рецепты › Обед»), заголовок в крошки не дублируется.
- **Pages nav (`.pages-nav__link`):** 14/600 `secondary`, `.is-current` — `primary`; **пагинация `.pages__pager`** — две `.btn-secondary` «Назад» / «Дальше» и счётчик `N / M` в `.num` по центру.
- **Section «more» (`.rpage__more`):** 14/600 ссылка справа от заголовка раздела («Все обеды»).
- **Footer (`.pages-foot`):** 13px `tertiary`, одно предложение об источнике цен и две текстовые ссылки.
- **ActionBar (`.actionbar`):** фиксированная стеклянная панель, `z-index: 20`, содержимое ограничено `--r-sheet-max` (1120px на `.shell--wide`). Набор на чеке: печать · семья · повторить неделю · поделиться (все ghost, только иконки 20px) · В магазин (primary); в магазине: назад · сбросить · поделиться (ghost-иконки) · Собрать корзину (primary). Текстовая кнопка на панели одна, чтобы на 360px ничего не вылезало.

### Kid menu (`.kidmenu`, `.kidrows`, `.viewswitch`)
- Секция на подложке `--ds-fill-4` между неделей и линией отрыва: заголовок 15/700 с иконкой Baby синим, сумма справа `.num`; дни — сетка `34px | 1fr`, блюда — строки `58px | 1fr | auto`. `.kidrows` внутри дня — те же `.dish` на `--ds-fill-4` с подписью 12/600. `.viewswitch` («все / взрослое / детское») — сегмент-контрол 36px на `--ds-fill-3`, активная кнопка белая с `--ds-shadow-sm`; на ≥1024 ограничен 360px. В режиме магазина скрыто.

### Family, wants, select, notifications (round 11)
- **Member card (`.kid.member`):** та же карточка, что у ребёнка: имя в `.form-control` 40px с иконкой `User` синим, `.member__label` 13/600 `secondary`, аппетит — `.segmented--3` на `--ds-fill-4`, приёмы дома — `.chip--sm` с `aria-pressed` (все включены по умолчанию). Цели — на следующем шаге, по `.options` на человека (`.option--compact`, если участников больше одного), имя человека как `.quiz__label`.
- **Wants (`.quiz__group--sep` + `.chip--want`):** блок «Хочется на этой неделе» отделён линией `--ds-separator` от стоп-продуктов; выбранный продукт — пилюля выбранного цвета (tint, синяя рамка, синий текст 600) с крестиком, чтобы не путать с серыми `.chip--remove` исключений.
- **Eaters on receipt (`.eaters`, `<ul>`):** под мета-строкой шапки, отделены `--ds-fill-4`; строка на человека `minmax(64px,max-content) | 1fr`: имя 600 `primary`, справа цель, `N` ккал/день и `×f` порции в `.num`, «дома: …» — 13 `secondary`, `gap 12px`, без точек-разделителей. Показывается только для неоднородной семьи. `.eaters__family` — строка «Семья: …» с иконкой `Users` 13px. В дне — `.day__members` 12 `tertiary`: имя 600 + ккал `.num`.
- **Invite:** иконка `Users` как `.btn-ghost` рядом с печатью в панели действий; primary остаётся один («В магазин»).
- **Select (`.apple-select`, `components/Select.tsx`):** разметка DS: триггер 48px/16px, панель в `<body>` (`position: fixed`, `--ds-shadow-xl`, `--ds-radius-md`), поиск от 8 пунктов, опции 40px 15px, выбранная — tint + синий + `Check` 16px. Обводки нет: открытый триггер — только рамка `--ds-blue`, кольцо `--ds-shadow-focus` лишь с клавиатуры; поле поиска в фокусе — рамка `--ds-separator` на `--ds-fill-4`. Заменяет все `<select>`.
- **Notify card (`.notify`):** `.account__card` в колонке профиля: заголовок 15/600 с `Bell` синим, статус «Включены» 12/600 `--ds-green` (DS-семантика «включено», как у `.switch`), кнопка включения — `.btn-soft` (не второй primary), настройки — два `Select` в `.notify__pair` (`1.4fr | 1fr`) и два `.switch`, «Проверить» — `.btn-ghost`.
- **Own recipe form (`.ownform`):** карточка `--ds-radius-lg`, поля `.auth__field`, приём — `.segmented--4`, техника и теги — `.chip--sm`, продукты — строки `1fr | 128px | 36px` с единицей внутри поля (`.ownform__unit` 13 `tertiary`), шаги — `<textarea>` по строке на шаг, ошибка — `.ownform__error` на `--ds-red-tint`. Один primary на форму («Добавить в мои рецепты»), «Отмена» — `.btn-soft`.
- **Partner row (`.partner`, `components/PartnerRow.tsx` и тот же блок в `recipe.html`):** единственный формат рекламы. Одна строка под шагами рецепта над `--ds-separator`: подпись 11/600 uppercase `label-tertiary` («Где купить», с партнёрским флагом — «Реклама») → ссылки 13/500 `label-secondary` «Аэрогриль <span class="partner__shop">Яндекс Маркет</span>» с `ExternalLink` 12px (hover — `blue`) → `erid` 11 `.num`, справа крестик 24px «скрыть на 30 дней». Без фона, без карточки, без изображений товаров: читается как сноска. Появляется только для техники, которую покупают (аэрогриль, мультиварка, пароварка, гриль, блендер, миксер, мясорубка), и только если у страны есть магазин. В корзине партнёрская доставка помечена `.partner__note` («Реклама») над списком. Ничего рекламного в чеке, квизе, шапке и на главной.
- **Store monogram (`.store-logo__mono`):** для сетей без свободного SVG — квадрат 26px `--ds-radius`-7 в фирменном цвете сети с двумя буквами `#fff` 11/800; единственное место с брендовыми hex (они — цвет чужой марки, не палитра поверхности).

### Round 15: welcome, home pantry, cooking, feedback, cart, budget, photos, admin
- **Welcome sheet (`.welcome`):** first-visit `.bsheet` with 24/700 title, lead, four rows `44px icon tile (blue-tint, radius 12) · 16/600 title + 14 secondary text`, then `btn-primary btn-lg` «Начать» and a `btn-link` «Пропустить» in secondary colour. Shown once.
- **Home pantry chips (`.chip--have`):** green-tint chip with `Home` icon; same search-and-chips block as «хочется», separated by `.quiz__group--sep`. In the receipt an item covered at home renders like a pantry row (`.item--pantry`, no cost) with «есть дома»; partial cover says «дома 500 г». Totals add `.totals__row--saved` with the saved sum in green.
- **Feedback on eaten dishes (`.dish__ask`):** two 28px icon buttons `ThumbsUp` / `ThumbsDown` replace swap/dislike on past days; the chosen one fills green-tint / red-tint with matching border. Push notification carries two action buttons with the same labels.
- **Day away (`.day__skip`, `.day--away`):** 28px calendar-off icon in the day header; away day collapses to header with «Не дома» in tertiary and a calendar-plus icon to bring it back.
- **Move mode (`.movebar`, `.is-moving`, `.dish__here`):** a floating pill toast above the action bar (same material as `.toast`: label-primary fill, 14/500 inverted text, blue icon, underlined «Отмена»; content does not shift); the source row gets blue-tint background bleeding 8px into the gutters; target rows of the same slot show a `btn-primary btn-sm` «Сюда» (36px tall, 8px 14px padding) in the actions column.
- **Cook mode (`dialog.cook`):** full-screen, `bg-base`, grid `head / body / running timers / nav`. Head: 4px segmented progress + uppercase 13/600 «ШАГ 2 ИЗ 6» and a 36px close. Body: one `.cook__card` ≤ 640px, step number 15/700 blue, step text 24/500 (28 on ≥700px), timers as `btn-primary` with tabular numbers (green while running, red when done) plus a soft reset button. Nav: `btn-soft btn-lg` back + full-width `btn-primary btn-lg` next; safe-area padding. Final card centres an 80px green-tint `ChefHat` disc.
- **Cart sheet (`.cart`):** segmented store/delivery switch, rows `name + tertiary qty · btn-soft btn-sm «Найти» with ExternalLink`, actions row with `btn-primary` «Скопировать список» and a tertiary hint.
- **Budget chart (`.budget`):** elevated card, 16/700 title, legend dots; SVG bars per week (plan `fill-2`, bought `blue`, radius 3), 11px tertiary labels; footer line with `TrendingDown` in green or `TrendingUp` in orange.
- **Photo field (`.photo`):** 96px dashed square (64px circle for avatars) that previews the image; beside it `btn-soft btn-sm` «Добавить фото / Заменить», `btn-link` «Убрать» and a 12.5 tertiary hint. Comment rows show a 24px round avatar and an optional image ≤ 320px with md radius.
- **Confirm (`.confirm`, `components/Confirm.tsx`, `useConfirm()`):** replaces every native `confirm()`. Centred 320px card, radius 18, `bg-elevated`, blurred backdrop; 17/600 title, 14 secondary text, two equal 44px buttons: `btn-soft` cancel (autofocus) and `btn-danger` / `btn-primary` action. Scale-in 180ms.
- **Error pages (`.errpage`, SSR `error.html`, nginx `50x.html`, SPA `NotFound`):** `.state__box` ≤ 360px centred, 64px `fill-4` disc with a 28px icon (search-x / lock / triangle), code as 13/600 tracked tertiary, 22/700 title, secondary text, `btn-primary` «Рецепты» + `btn-soft` «На главную».
- **Catalog pager (`.pager`):** 36px number pills (`fill-4` on hover, current filled blue with white text), chevron arrows at both ends, gaps as tertiary ellipsis. Country of prices is a chip group inside the filters and an underlined hint after the lead («Цены для страны: … — сменить»).
- **Comment editor (`.mdbar`):** 32px icon buttons (bold, italic, strike · list, quote, link · photo), separators 1px `separator`; photo button shows a 24px thumb after upload. Rendered comments get real paragraphs, disc/decimal lists, a 3px `fill-2` blockquote rule, inline code on `fill-4`, blue underlined links.
- **Own-recipe status (`.planrow__status`):** one 12.5px line under the row: blue while checking, orange for manual review, red rejected, green published.
- **Moderation card (`.modcard`):** elevated card, title link + «@author · slot · min · N h in queue», optional 72×54 thumb, AI reason on `orange-tint`, steps as an ordered list, ingredients as tertiary one-liner, then `input | btn-primary «Опубликовать» | btn-danger «Отклонить»`; cards older than 20 h get an orange border.
- **Purchase receipts (`.preceipts`, `.preceipt`):** history as a stack of small receipts. Month sections with a 16/700 title and a tertiary «N походов · M позиций · сумма» line; a grid `minmax(240px, 1fr)` of cards: mono 12.5px body, sans header with a 24px blue-tint bag icon, date in bold and the item count on the right, dashed rules, dotted leaders between name and price, «ещё N» as a blue 13/600 link with a rotating chevron, «Итого» footer, torn bottom edge done with a radial-gradient mask (`--edge: 8px`). Two latest months are shown; «Показать ещё месяцы (N)» is a `btn-soft`.
- **Admin lists (`.alist`, `components/AdminList.tsx`):** no tables. A row is `media · title + meta + tag chips · number · actions` in an elevated card (radius lg, 10px 12px): recipe rows carry a 56×42 thumb (or a slot-coloured gradient tile), monospace id on the right and a trash icon; account rows carry a 40px monogram/avatar, role badge (`.badge--admin` red-tint, `--moderator` blue-tint), the plan count as `17/700 + 11px label`, and an inline role Select. On ≤560px the number and actions drop to a second full-width line. Events and tops are `.abar` rows: label, bold count, tertiary hint and a 4px blue share bar. Tabs are `.chip` pills with icons.
- **Suggestion card (`.suggest-card`):** blue-tint block under an own recipe: bold blue title «Подробная версия от помощника», the AI reason, proposed title/description/steps as an ordered list, then `btn-primary btn-sm` «Принять и опубликовать» + `btn-soft btn-sm` «Оставить как есть».
- **Admin (`.admin`, `.shell--wide` 1100px):** stat cards (12.5/600 label, 24/700 value), day bars (visitors `fill-2`, plans blue, users green), plain tables with uppercase 12px headers, errors as `<details>` cards, logs monospace 12.5 in a scrollable elevated box with level colouring. Operate mode: no decoration beyond the card grid.

### Round 16: occasions, collections, substitutions, ingredient pictures, PWA, footer
- **Occasion header (`.occ`):** the receipt for a single table (not a week) opens with the event name 22/700, a lead in secondary and course groups «Салаты · Закуски · Горячее · Десерт» as 13/600 uppercase tracked labels; dishes keep the `.dish` row, swap stays inside the course. Quiz shows events as `.chip` pills with lucide icons before the presets.
- **Collections in the cabinet (`.coll`):** each collection is a `.planrow` with a `FolderOpen` icon, «N рец.» meta and an actions cluster: `btn-soft btn-sm` «Неделя», then 28px icon buttons link (blue-tint when public), pencil, trash and a rotating chevron; expanded items are nested `.planrow` links with an `X`. The «В коллекцию» button lives in `SocialBar` and opens a `.bsheet` picker with checkboxes and a «новая коллекция» field.
- **Curated strip (`.colstrip`, `.colcard`):** at the top of the catalog, «Подборки» 18/700 and a horizontal snap row of 16:9 cards (grid on ≥900px): cover photo (or a blue→purple gradient), bottom gradient with 16/700 white title and 12px count. Hover scales the image 1.03.
- **Collection page (`.colhead`):** breadcrumbs, then a 5/7 split — 3:2 cover with xl radius and a body of 28/700 title, 16px secondary lead, 13px tertiary «N рец. · @author» and `btn-primary` «Собрать неделю из коллекции»; recipes follow in the usual `.rgrid`.
- **Admin collections (`AdminCollections.tsx`):** `.alist` rows with the cover thumb (`coverAuto`), a `.badge--moderator` «черновик» for hidden ones, eye / eye-off on the right and a trash action; the form is the `.ownform`: name, slug with placeholder `new-year-table`, description, `PhotoField` cover, `.switch` «Показывать на сайте», picked recipes as `.ownform__ing` rows with `X`, `.search` + `.suggest` listbox to add.
- **Substitutions (`.subs`, `.subs__pop`):** an `ArrowLeftRight` 14px icon after an ingredient that has substitutes; click opens a small popover (`bg-elevated`, radius md, shadow sm, ≤ 280px) with «Чем заменить» 13/600, one row per option: name, amount, delta «+40 ккал · −12 ₽» in tertiary with green/red numbers and the note in 12.5px.
- **Ingredient pictures (`.has-pic`, `.ings__pic`):** ingredient names with a picture get a dotted underline; hover/focus shows a 120px square WebP with radius md and shadow md above the name, no layout shift.
- **Language dialog (`.langdlg`, `.langlist`):** the same bottom sheet in SPA and SSR (`.bsheet__body` wraps the content so it scrolls and keeps the 20px gutter); languages as 48px rows `28×21 flag · 16/600 name + 12.5 English name`, two columns from 600px, the current one on blue-tint with a blue border.
- **Substitution popover (`.subs__pop`), revised:** each option is `bold name · amount` on the first line, a plain-language note on the second («Кислинка ярче, жира меньше. В горячее класть в конце, иначе свернётся»), and the delta «+10 ккал · −6 ₽» in tertiary on the third. Notes are instructions, not adjectives.
- **Sheet close button (`.bsheet__close`):** dark glass (rgba 20/20/24 at 55 %, blur 8px, 1px white 18 % ring) with a white icon, so it stays legible over a recipe photo in both themes.
- **Own-recipe rows (`.planrow__stats`, `.planrow__publish`, `.planrow__actions`):** a private recipe shows `btn-soft btn-sm` «Опубликовать» with an Upload icon; a published one shows a 12.5px tertiary stats line `eye N · thumbs N · heart N · comment N` under the status, and the link icon. Actions sit in a flex group; on ≤560px the row stacks and the group aligns right.
- **Dishes on phones (≤640px):** 13px vertical padding, `separator` rule instead of `fill-4`, and the planner reason («— дешевле цели · легче нормы») drops to its own single ellipsised line under «Ужин · 20 мин».
- **Collections page (`/collections`):** the same `.colcard` grid with the description clamped to two lines under the count; the catalog strip shows eight cards and a blue «Все подборки (N)» link at the right of the title.
- **Translation line (`.trline`, `.trchip`):** under an own recipe, a 12.5px tertiary row `spinner/globe · «Переводы: 5 из 14 · сейчас Español · mistral/ministral-14b-latest» · chevron`; expanded, language chips in pill form — green-tint done, blue-tint running with a spinner, red-tint error with the message in `title` — and a footer «Переводит: provider/model, …» with a `btn-link` retry. Recipes without a queue show a single «Перевести на другие языки» link in the same row.
- **AI providers card (admin overview):** `.abar` rows `name <small>model</small> [badge отдыхает]`, the bar is today's requests against the daily quota (or against itself when unlimited), hint «N из M запросов за день» or «N за день · удачных X, ошибок Y».
- **Offline (`.offline`):** a 32px orange-tint strip under the header, `WifiOff` 14px + 13/500 «Нет сети: отметки сохранятся и отправятся позже».
- **Footer (`.sitefoot`):** top separator, four columns on desktop (brand + one line about the product; «Рецепты», «Аккаунт», «О проекте» link lists in 14px secondary with 8px gaps), the language button `[data-lang-btn2]` on the right; on phones columns stack in two, language goes last. Bottom line 12.5 tertiary with the Rosstat note.

### Account, extras
- **Вход/кабинет** (`.auth`, `.account`): те же поля `.form-control` 48px, сегмент-контрол на две/три вкладки (`.segmented--3` с иконкой lucide 16px), строки `.planrow`, история покупок `.purchday`/`.purchrow` в три колонки с `.num`, «Не предлагать» — те же `.planrow` со ссылкой на `/recipe/{id}`.
- **Свои товары** (`.list__group--extras`): та же строка `.item` с чекбоксом, справа `.item__del`; форма `.extra-form` из трёх полей под кнопкой `.btn-soft.btn-sm`.
- **Не предлагать**: у блюда две вертикальные иконки `.dish__actions` (замена и палец вниз, hover красный); гость уходит на `/login?plan=`.

### Price provenance
- Под суммами шапки чека — `.receipt__source` 12.5px `tertiary` (источник, регион, период, индекс сети). В списке у позиции с ценой Росстата — зелёная точка `.item__src` 6px после названия. На рецепте источник назван в панели «Стоимость» (`Средние цены Росстата по России, {период}. В магазине будет иначе…`, при наличии — «Дороже всего здесь …») и в примечании к упаковкам `.ings__name small` («упаковка 900 г ≈ 276 ₽», «≈ 49 ₽ за кг»).
- Итоги чека: строка «Бюджет: … → ≈ факт» — факт зелёный, если в пределах +5% от цели, иначе `--ds-orange`; «Из них детское питание» появляется только при детях на смеси/прикорме.

### Recipe sheet anatomy (signature)
Лист рецепта — один документ: `.rpage__hero` (крошки → H1 → подводка → четыре факта → теги) → `.rpage__cols` → `.rpage__main` («Продукты» + `.portions` → `.ings` → примечание → «Как готовить» `.steps` → «Коротко» `.facts`, если есть что сказать) / `.rpage__aside` («Пищевая ценность» → «Стоимость» → «Понадобится» → CTA) → `.rpage__foot` «Похожие» (`.rgrid--tight` из `.rcard`).

- **`.ings__row`:** `<li id="ing-N">` — `.ings__n` номер 12 моно `tertiary` · `.ings__name` 16/400 с `small` 12 `tertiary` (примечание про упаковку или цену за кг) · `.ings__qty` 16/500 `secondary` `.num` с `data-amount`/`data-unit`. Кладовые (`.ings__row--pantry`: соль, масло, специи) — название `secondary`, количество `tertiary`; продукт без граммовки (меньше грамма) пишется словами «по вкусу» без `.num`. Примечание под списком объясняет серый цвет и что кладовые не входят в цену.
- **`.steps__item`:** CSS-счётчик в кружке 30px на `--ds-fill-3` (14/600 `secondary`) · `<p>` 17/1.5 с инлайн-ссылками `.steps__ref` на строки продуктов. Никаких заголовков у шагов, никаких списков продуктов под шагом.
- **`.facts` («Коротко»):** маркированный список 15/1.45 `secondary`, маркеры `quaternary`; рендерится только если есть факты, которых нет в плитках шапки (плотность на 100 г, «готовится сразу на два дня», самый дорогой продукт); пустой или дублирующий раздел не выводится.

### Receipt row anatomy (signature)
Чек — один документ: `.receipt__head` → `.receipt__warn` → `.receipt__menu` из `.day` → `.dish` → `.tear` → `.receipt__list` / `.list` → `.totals`.

- **`.dish`:** три колонки. `.dish__main` — кнопка на всю строку, внутри `dish__title` 16/600 и `dish__why` 12.5px `tertiary`: слот приёма пищи первым словом (`secondary`, 500, «·» после), `Clock` 13px и минуты, затем «— почему выбрано». `.dish__nums` справа: `≈ ₽` 14/600 и `Flame` 11px + ккал 13, оба `.num`. `.dish__swap` — 36px ghost с `RefreshCw` 16px.
- **`.list`:** заголовок 20/700 + счётчик позиций; группы по категории с uppercase-лейблом и `≈ ₽` группы; `.item` — круглый `role="checkbox"` 22px, название 15/500, количество `.num` 13, цена `.num` 14.
- **`.totals`:** черта 2px `label-primary`, строки 14px `secondary`; главная 20/700 с зелёной суммой; КБЖУ в `--stack`; примечание 12px `tertiary`.

### Tear line and pull gesture (signature)
`.tear` (`role="separator"`) — 32px полоса, `border-top: 2px dashed --ds-separator-opaque`, два круглых выреза 20px цвета грунта по краям, в центре пилюля `.tear__btn` «Оторвать список». Жест: pointer capture, тянуть только вверх до 140px, порог 48px; переход в режим магазина через `document.startViewTransition` с fallback.

### State vocabulary
- **Selected** — `aria-checked="true"` / `aria-pressed="true"` / `.is-on`: синяя рамка + `--ds-blue-tint` + inset-кольцо 1px (чипы фильтров в SSR — то же через `aria-pressed`).
- **Current** — `.pages-nav__link.is-current`: `label-primary` вместо `secondary`; `.theme-btn.is-user`: синий.
- **Done / checked** — `.item.is-done`: зелёный кружок с белой галочкой, название зачёркнуто `tertiary`.
- **Pantry** — `.item--pantry` / `.ing--pantry` / `.ings__row--pantry`: название `secondary`, цена не показывается.
- **Leftover** — `.dish--leftover`: название `secondary` 500, `Repeat2` + «вчерашнее, готовить не нужно», без кнопки замены.
- **Fresh (reprint)** — `.dish.is-fresh` после замены и `.ings__qty.is-fresh` после смены порций: анимация `reprint` 600ms — фон от `--ds-blue-tint` к прозрачному. Перепечатывается только изменившееся.
- **Busy** — `.dish__swap.is-busy`: `spin` 720ms; `aria-busy="true"` на чеке/рецепте во время загрузки.
- **Printing** — `.receipt.is-printing` при первом показе: каскад `print` 420ms по `--ds-ease-emphasis`. Единственный авторский момент анимации в продукте.
- **Open** — `.filters[open]`: уголок поворачивается на 180°.
- **Disabled** — opacity 0.35–0.45, `cursor: default`.

### Motion
Всё из DS: `--ds-dur-fast` 140ms / `base` 220ms / `slow` 360ms; `--ds-ease-spring` для transform и box-shadow (в том числе hover карточки рецепта), `--ds-ease-standard` для цвета и фона, `--ds-ease-emphasis` только для печати чека. Нажатие — `scale(0.94–0.985)`. `prefers-reduced-motion: reduce` схлопывает все animation/transition до 0.01ms. SSR-страницы несут один инлайн-скрипт на порции и один на тему; библиотек нет.

## Do's and Don'ts

### Do:
- Все подписи — через `t()` (React) или `{{t .L "key"}}` (SSR): интерфейс живёт на ru/en/de, текст в JSX и шаблонах не пишется. Деньги — `money(v, country, lang)`: валюта страны плана (₽, Br, ₸, €, $), числа в локали интерфейса. Страна выбирается чипами на первом шаге квиза; магазины и пресеты бюджета зависят от неё.
- **Do** брать каждый цвет, радиус, тень и easing из `--ds-*` (`apple-tokens.scss`); поверхность добавляет только `--r-sheet-max`, `--r-gutter`, `--r-bar-h`, `--r-tabular`, `--r-safe-bottom` и вычисляемый `--rp-tint`.
- **Do** оборачивать любое число в `.num` (JetBrains Mono, `tnum lnum`) и выравнивать его по правому краю колонки — в React и в Go-шаблонах одинаково.
- **Do** выводить оценки через `approxRub()` / `rub` — «≈ 7 994 ₽»; красить в `--ds-green` только их (`.is-rub` вне чека); без «≈» только целевой бюджет пользователя.
- **Do** держать один `.btn-primary` на страницу и одну приподнятую поверхность (чек / лист рецепта / карточка входа); внутри листа разделять линиями `--ds-fill-4` / `--ds-separator` и тональными панелями `--ds-fill-4`.
- **Do** ставить тинт приёма пищи только как подложку: градиент шапки `.rpage--{slot}` и полоска 4px `.rcard__cover--{slot}`.
- **Do** помечать продукт в тексте шага инлайн-ссылкой `.steps__ref` с надстрочным номером строки; нумерация шага — CSS-счётчик, нумерация продукта — `.ings__n`, номера совпадают.
- **Do** давать интерактивам ≥44px (плитки 88, чипы 44, счётчик 44, опции 64, кнопки панели 48, поля 48; иконки топбара 36, порции 36, замена 36 и `.chip--sm` 34 — исключения внутри строки/шапки), `:focus-visible` с `--ds-shadow-focus`, роли `radio`/`checkbox`/`progressbar`/`separator`/`listbox`/`group`/`search`, `aria-label` на иконочных кнопках, `aria-hidden` на иконках, `aria-labelledby` на разделах листа.
- **Do** использовать lucide (13–22px, strokeWidth 2–3) для любой пиктограммы: `lucide-react` в React, те же контуры инлайн-SVG в шаблонах; иконка стоит перед текстом с зазором 5–6px.
- **Do** писать по-русски, коротко и по делу: заголовки разделов — одно слово («Продукты», «Как готовить», «Коротко», «Похожие», «Стоимость»), подписи объясняют метод, не хвалят; склонять счётчики через `plural`.
- **Do** уважать `prefers-reduced-motion`, `env(safe-area-inset-bottom)` и тёмную тему: `[data-theme]` на `<html>` из `localStorage["racion.theme"]` (один и тот же ключ и цикл в React и SSR), `<meta name="color-scheme">` и два `theme-color`; поверхность не переопределяет ни одного тёмного значения.
- **Do** делать боковую панель рецепта липкой только при `≥1024px` и `min-height: 760px`; иначе она скроллится с листом.
- **Do** давать стилю печати снять топбар, панель, кнопки замены/отрыва и чекбоксы — чек печатается как чек.

### Don't:
- **Don't** вводить hex или новые цвета в `app.scss`; не красить кнопки зелёным, не красить «в бюджете» / «дороже цели» — эти слова остаются `label-tertiary`; не использовать `--ds-orange` / `--ds-purple` как текст или рамку.
- **Don't** ставить второй `.btn-primary` на страницу — ни в топбаре, ни в подвале, ни в шапке листа; навигация там только текстом и иконками.
- **Don't** дробить результат на вкладки «Меню / Список / КБЖУ» или показывать фейковый лоадер — загрузка это skeleton внутри той же карточки.
- **Don't** ставить эмодзи, иконочные шрифты, растровые иконки или системный шрифт вместо самохостных Inter / JetBrains Mono; в шаблонах — только инлайн-SVG.
- **Don't** выносить слот приёма пищи («Завтрак») в эйрброу над названием блюда; на карточке рецепта слот — первое слово мета-строки, на странице рецепта — второе звено крошек.
- **Don't** выводить продукты шага отдельным списком, чипами или в квадратных скобках — только `.steps__ref` внутри предложения.
- **Don't** повторять в «Коротко» факты из плиток шапки; если сказать нечего — раздел не рендерится. Не писать «по вкусу» в `.num` и не выводить сотые грамма.
- **Don't** делать вложенный скролл внутри чека или боковой панели рецепта; не прилипать панелью на окнах ниже 760px.
- **Don't** рисовать цветную обложку с текстом у карточки без фото — только полоска 4px; не показывать `≈ ₽` серым.
- **Don't** добавлять бумажные детали кроме линии отрыва: без зубчатых краёв, текстур, тени-«листка», перфорации.
- **Don't** показывать линию отрыва на десктопе или в режиме магазина.
- **Don't** заменять кольцо фокуса на `outline` и не совмещать кольцо фокуса с inset-кольцом выбора в одном `box-shadow`.
