---
version: 1
slug: "frontend-src-app-tsx"
primary_target: "frontend/src/App.tsx"
related_targets: ["frontend/src/pages/Quiz.tsx","frontend/src/pages/Plan.tsx"]
---

# Surface: Рацион — квиз → чек (frontend/src/App.tsx)

Scope: whole product surface, mobile-first web. Visitor mode: **Operate** (complete a task: get a week menu and a shopping list).

Audience/job: home cook in Russia, phone in hand, wants the week decided and the list ready. Action: answer 6 questions, receive the plan, take the list to the store, swap a dish when needed. Proof/content: real recipe base, gram amounts, per-day KBJU, estimated cost labelled as estimate.

Constraints: Rubby Design System is binding (tokens, type scale, radii, spring motion). No emojis; SVG icons only. No fake progress, no paywall, no referral links. Russian only.

Unresolved: dark-mode default (system preference for now), domain/logo.

## Direction contract

THESIS: The result is one continuous document — a receipt — not a dashboard. It refuses the category's tabbed "Меню / Список / КБЖУ" result and the fake "собираем рацион…" loader.

OWN-WORLD: Rubby tokens: `--ds-bg-base` ground, one elevated white sheet, blue accent only on the primary action, green only on totals. Mono numerals (`--ds-font-mono`, tabular) in a right-aligned column; dish names lead in weight. A dashed tear line separates menu from list. Radii `--ds-radius-lg`, motion `--ds-ease-spring`.

STORY: "Six taps, and the week is on one sheet; the list is the bottom half of the same sheet." The visitor trusts the numbers because they are labelled estimates and every dish is a real recipe one tap away.

FIRST VIEWPORT: Quiz step 1 fills the phone: eyebrow "Шаг 1 из 6", question as title-1, 3×3 store tiles (≥44px, icon + name), sticky bottom primary button "Дальше". Result first viewport: sheet header (store · people · week), then day ПН with its dishes and a right-aligned mono ≈₽ column, the next day peeking below.

FORM: Receipt/чек — index 6 of my ordered list, dealt lead. Seed key **cd59ffcb**. Signature interaction: the tear — pulling the list up along the dashed line switches the sheet into store mode (checkboxes, larger rows); swap on a dish reprints only that line.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.
