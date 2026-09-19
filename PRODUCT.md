# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Go (backend, planner, PostgreSQL via pgx) + React/TypeScript (Vite) + Docker Compose + PostgreSQL. User decision. Frontend styles come from the Rubby Design System (Bootstrap 5.3 + Apple-HIG SCSS layer, `C:\rubby\01_hrms\vendor\design-system`, MIT), vendored into `frontend/src/styles/design-system/`.

## Users

A person in Russia who cooks at home for themselves or their family, shops in a mainstream grocery chain (Пятёрочка, Магнит, Лента, Ашан, Дикси, Метро, Перекрёсток, Светофор, ВкусВилл), and wants two things: stop deciding "what to cook" every day and spend less on groceries. Mobile-first: they open it on a phone, often at the store or in the kitchen. Secondary: the same person a week later, regenerating or swapping dishes.

## Product Purpose

«Рацион» turns a two-minute questionnaire (store, who eats, allergies, disliked foods, kitchen equipment, meals per day, budget) into a seven-day menu with full recipes, per-day calories/protein/fat/carbs, and a consolidated shopping list with quantities, pack rounding and an estimated cost. Success: the person walks into the store with the list and cooks the week without opening anything else.

## Positioning

Free, no account, no paywall, no referral links: the result is shown in full immediately after the last question. A commercial neighbour (legkost-test.ru/racion, 590 ₽/month, result hidden behind payment) cannot truthfully copy this. The plan is deterministic and reproducible: same answers → same plan; a swap is a visible, explainable change, not a black box.

## Operating Context

- Phone in hand, one thumb, at home or in a store aisle. Bright store light and dim kitchen light both happen.
- The shopping list is used *inside* the store: items get checked off, grouped by aisle category.
- Recipes are read while cooking: steps must be scannable at arm's length.
- A plan has a shareable link (stored server-side by id) so it can be sent to a partner.

## Capabilities and Constraints

- Recipe base is our own curated JSON (real Russian home dishes, gram amounts, per-100g nutrition, price tier), seeded into PostgreSQL. No LLM generation at runtime.
- Planner is deterministic Go code: filters by allergens/exclusions/equipment/slots, scores by budget fit, variety and ingredient reuse (less waste), seeded RNG for tie-breaks.
- Store prices are an **estimate**: base average price per ingredient × per-chain index stored in DB. The UI must say "оценка, не ценник" wherever a ruble figure appears. No live prices exist and none may be implied.
- Analytics: Yandex Metrika counter (id from env) + first-party event analytics in PostgreSQL (anonymous session id, event name, step, timestamp). No referral/affiliate tracking, no partner links.
- No emojis anywhere in UI. Icons are SVG only (lucide-react).
- Language: Russian only for now.
- Undecided: domain, logo mark, dark-mode default (design system supports both), account/login (out of scope for v1).

## Brand Commitments

- Name: **Рацион**.
- Visual system: Rubby Design System (Apple HIG on Bootstrap 5.3) is binding — its tokens, type scale, radii, motion, components. This surface must read as a sibling of the rubby app family.
- Tone: direct, concrete, no hype, no gamification, no fake progress screens, no countdown timers. "Чёткость."

## Evidence on Hand

- Competitor teardown: legkost-test.ru/racion quiz structure and copy (session notes). Use as anti-reference for funnel tricks; its step *order* (store → eaters → allergies → stop-foods → kitchen → meals) is a sound UX sequence and may be kept.
- No testimonials, no user numbers, no press. Nothing of that kind may be invented.
- Recipe and price data authored in-repo (`backend/seed/`); prices are approximate and dated in the file.

## Product Principles

1. The result is the product: show the full plan and list immediately, never gate it.
2. Every number is honest: costs are labelled estimates; calories come from the ingredient table, not marketing.
3. Deterministic and explainable: a swap says what changed and why a dish qualified.
4. Built for one thumb in a store aisle: large targets, checkable list, offline-tolerant state.
5. Same family as the rubby apps: the design system is inherited, not reinterpreted.

## Accessibility & Inclusion

Keyboard-navigable quiz, visible focus rings (design system pattern), ≥44px touch targets, WCAG AA contrast on both themes, `prefers-reduced-motion` respected. Allergen filtering is a safety feature: allergen data on ingredients must be complete for the eight common allergens used in the quiz.
