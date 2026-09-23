#!/usr/bin/env node
// MCP-сервер «Рацион»: планирование недели с ценами магазина и списком покупок.
// Транспорт stdio — так его подключают Claude Desktop, ChatGPT и прочие клиенты.
// Ключей нет: сервер ходит в публичный API racion.app, описанный в /openapi.json.

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";
import { api, ApiError, BASE, type Country } from "./api.js";
import { planText, recipesText, recipeText } from "./format.js";

const LANGS = ["ru", "en", "de", "fr", "es", "it", "pt", "pl", "uk", "tr", "nl", "cs", "kk", "zh", "ja"] as const;
const SLOTS = ["breakfast", "lunch", "dinner", "snack"] as const;
const EQUIPMENT = ["stove", "oven", "microwave", "airfryer", "multicooker", "blender", "mixer", "grill", "steamer", "meatgrinder"] as const;

const server = new McpServer({ name: "racion", version: "1.0.0" });

// Страна нужна почти каждому ответу, чтобы показать валюту: держим её в памяти процесса.
let metaCache: { country: string; countries: Country[] } | null = null;
async function countryOf(code = "RU"): Promise<Country | undefined> {
  if (metaCache?.country !== code) {
    const m = await api.meta(code);
    metaCache = { country: code, countries: m.countries };
  }
  return metaCache.countries.find((c) => c.code === code);
}

function fail(e: unknown): { content: { type: "text"; text: string }[]; isError: true } {
  const msg = e instanceof ApiError ? e.message : (e as Error).message;
  return { content: [{ type: "text", text: `Racion: ${msg}` }], isError: true };
}

const country = z.string().length(2).default("RU").describe("Two-letter country code: it sets the currency and the price source. RU, DE, US, GB and 18 more.");
const lang = z.enum(LANGS).default("en").describe("Language of recipe titles and steps.");
const store = z.string().describe("Store code from racion_stores, for example pyaterochka, rewe, lidl, tesco.");

server.registerTool(
  "racion_stores",
  {
    title: "Countries and stores",
    description:
      "List the supported countries and the stores of one of them, with the price index each store applies to national average prices. Call this first if you do not know which store code to pass to the planner.",
    inputSchema: { country: country.optional(), lang: lang.optional() },
  },
  async ({ country: c = "RU", lang: l = "en" }) => {
    try {
      const m = await api.meta(c, l);
      const own = m.stores.filter((s) => s.country === c);
      const cy = m.countries.find((x) => x.code === c);
      const lines = own.map((s) => `- ${s.name} (${s.code}) — ${s.kind}, price index ${s.priceIndex}`);
      const all = m.countries.map((x) => `${x.code} ${x.symbol}`).join(", ");
      return {
        content: [
          {
            type: "text",
            text: `Stores in ${cy?.label ?? c} (prices in ${cy?.currency ?? "?"}):\n${lines.join("\n")}\n\nOther countries: ${all}`,
          },
        ],
        structuredContent: { countries: m.countries, stores: own },
      };
    } catch (e) {
      return fail(e);
    }
  },
);

server.registerTool(
  "racion_search_recipes",
  {
    title: "Search recipes",
    description:
      "Search 1300 home recipes by words in the title or ingredients. Every result carries the time, calories and the estimated cost of one portion in the currency of the chosen country.",
    inputSchema: {
      query: z.string().optional().describe("Words to look for: chicken, pumpkin soup, pizza dough."),
      slot: z.enum(SLOTS).optional().describe("Keep one meal only."),
      country: country.optional(),
      lang: lang.optional(),
      limit: z.number().int().min(1).max(50).default(20).optional(),
      offset: z.number().int().min(0).default(0).optional(),
    },
  },
  async ({ query, slot, country: c = "RU", lang: l = "en", limit = 20, offset = 0 }) => {
    try {
      const page = await api.recipes({ q: query, slot, country: c, lang: l, limit, offset });
      return { content: [{ type: "text", text: recipesText(page, await countryOf(c)) }], structuredContent: page };
    } catch (e) {
      return fail(e);
    }
  },
);

server.registerTool(
  "racion_get_recipe",
  {
    title: "Recipe with ingredients and steps",
    description: "One recipe in full: ingredients per portion in grams, steps, calories and macros, and the estimated cost of a portion.",
    inputSchema: { id: z.string().describe("Recipe id from the search, for example syrniki."), country: country.optional(), lang: lang.optional() },
  },
  async ({ id, country: c = "RU", lang: l = "en" }) => {
    try {
      const r = await api.recipe(id, c, l);
      return { content: [{ type: "text", text: recipeText(r, await countryOf(c)) }], structuredContent: r as unknown as Record<string, unknown> };
    } catch (e) {
      return fail(e);
    }
  },
);

server.registerTool(
  "racion_plan_week",
  {
    title: "Plan a week of meals",
    description:
      "The main tool. Turn who eats, where they shop and what they cannot have into seven days of dishes plus one shopping list rounded to whole packs, with an estimated total. The planner is deterministic: the same answers give the same week, and each dish says why it was chosen. Give the person the returned link — it opens the plan as a page they can print, tick off in the shop and share.",
    inputSchema: {
      country: country.optional(),
      store,
      lang: lang.optional(),
      adults: z.number().int().min(1).max(10).default(2).optional(),
      kids: z.array(z.number().int().min(0).max(17)).optional().describe("Ages of the children eating; portions and spice are adjusted for them."),
      slots: z.array(z.enum(SLOTS)).optional().describe("Which meals to plan. Default: breakfast, lunch, dinner."),
      equipment: z.array(z.enum(EQUIPMENT)).optional().describe("What the kitchen has. Default: stove, oven, microwave."),
      excludeTags: z.array(z.string()).optional().describe("Whole groups nobody eats: meat, poultry, fish, seafood, offal, spicy."),
      allergens: z.array(z.string()).optional().describe("Allergen ids from racion_stores; dishes containing them are never offered."),
      budgetPerPersonDay: z.number().min(0).optional().describe("Target spend per person per day in local currency. 0 or absent means no target."),
      collection: z.string().optional().describe("Collection slug whose dishes should be taken first, for example pizza-home or childhood-food."),
      region: z.string().optional().describe("Region or city where prices differ; empty means the national average."),
    },
  },
  async (a) => {
    try {
      const c = a.country ?? "RU";
      const plan = await api.plan({
        country: c,
        store: a.store,
        region: a.region ?? "",
        lang: a.lang ?? "en",
        adults: a.adults ?? 2,
        kids: (a.kids ?? []).map((age) => ({ age, appetite: "normal" })),
        slots: a.slots ?? ["breakfast", "lunch", "dinner"],
        equipment: a.equipment ?? ["stove", "oven", "microwave"],
        excludeTags: a.excludeTags ?? [],
        allergens: a.allergens ?? [],
        exclude: [],
        budgetMode: "perPersonDay",
        budgetValue: a.budgetPerPersonDay ?? 0,
        collection: a.collection ?? "",
      });
      return { content: [{ type: "text", text: planText(plan, "Week of meals") }], structuredContent: plan as unknown as Record<string, unknown> };
    } catch (e) {
      return fail(e);
    }
  },
);

server.registerTool(
  "racion_build_table",
  {
    title: "Build a table from dishes you picked",
    description:
      "Give recipe ids and how many people each dish feeds; get one shopping list with prices for exactly that set. Made for holidays: three salads for eight, a chicken for four, a cake for twelve. Up to 30 dishes.",
    inputSchema: {
      dishes: z
        .array(z.object({ id: z.string(), servings: z.number().int().min(1).max(40).optional() }))
        .min(1)
        .max(30)
        .describe("Recipe ids with the number of people each one feeds. Missing servings means the same as guests."),
      guests: z.number().int().min(1).max(40).default(4).optional(),
      country: country.optional(),
      store,
      lang: lang.optional(),
    },
  },
  async (a) => {
    try {
      const plan = await api.basket({
        guests: a.guests ?? 4,
        items: a.dishes.map((d) => ({ recipeId: d.id, servings: d.servings ?? 0 })),
        params: { country: a.country ?? "RU", store: a.store, region: "", lang: a.lang ?? "en" },
      });
      return { content: [{ type: "text", text: planText(plan, "Table") }], structuredContent: plan as unknown as Record<string, unknown> };
    } catch (e) {
      return fail(e);
    }
  },
);

server.registerTool(
  "racion_collections",
  {
    title: "Curated collections",
    description:
      "Ready sets of recipes: holiday tables, pizza doughs, childhood food, viral recipes, quick dinners. A slug from here can be passed to racion_plan_week so the week is built from that collection first.",
    inputSchema: { lang: lang.optional() },
  },
  async ({ lang: l = "en" }) => {
    try {
      const cols = await api.collections(l);
      const rows = cols.map((c) => `- ${c.name} (${c.slug})${c.recipes?.length ? `, ${c.recipes.length} recipes` : ""}`);
      return {
        content: [{ type: "text", text: `${cols.length} collections:\n${rows.join("\n")}\n\nPages: ${BASE}/collections` }],
        structuredContent: { collections: cols },
      };
    } catch (e) {
      return fail(e);
    }
  },
);

const transport = new StdioServerTransport();
await server.connect(transport);
