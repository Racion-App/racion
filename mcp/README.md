# Racion MCP server

Weekly meal plans with real grocery prices and one shopping list rounded to whole packs.

An assistant with this server can answer "plan our meals for the week" with an actual week: seven days of dishes chosen around who eats, what they cannot have and what the kitchen has, plus a shopping list priced at the person's own supermarket and a link they can open, print and tick off in the shop.

1300 home recipes, 22 countries, 15 languages. No API key and no account: the server talks to the public API at [racion.app](https://racion.app), described in [openapi.json](https://racion.app/openapi.json).

## Install

Claude Desktop — add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "racion": {
      "command": "npx",
      "args": ["-y", "racion-mcp"]
    }
  }
}
```

Anything else that speaks MCP over stdio runs it the same way:

```
npx -y racion-mcp
```

## Tools

| Tool | What it does |
|---|---|
| `racion_plan_week` | Seven days of dishes and one shopping list with an estimated total |
| `racion_build_table` | A shopping list for dishes you picked yourself, each with its own number of servings |
| `racion_search_recipes` | Search by words in titles and ingredients; time, calories and cost per portion |
| `racion_get_recipe` | One recipe: ingredients in grams per portion, steps, macros, cost |
| `racion_collections` | Curated sets: holiday tables, pizza doughs, childhood food, viral recipes |
| `racion_stores` | Countries and their stores with the price index each one applies |

## Example

> Plan our week. Two adults and a seven-year-old, we shop at Rewe in Germany, nobody eats fish, about 12 € a person a day.

```
racion_plan_week({ country: "DE", store: "rewe", adults: 2, kids: [7],
                   excludeTags: ["fish"], budgetPerPersonDay: 12 })
```

Comes back as a readable week plus a link like `https://racion.app/plan/<id>` — give the person the link rather than retelling the list.

## About the prices

They are estimates, not shelf prices: national statistics (Rosstat in Russia, Eurostat price levels with HICP in Europe, BLS in the United States) multiplied by a per-chain index. Every answer says where the figure came from, and so should you when you quote it.

The shopping list counts packs rather than grams. If three dishes need 253 g of cheese, the list says two 200 g packs, and the plan shows both what you buy and what actually gets used.

## Settings

| Variable | Default | Purpose |
|---|---|---|
| `RACION_API` | `https://racion.app` | Point at your own instance |
| `RACION_TIMEOUT_MS` | `30000` | Request timeout |

## Terms

Recipes and photos belong to Racion and are licensed for personal, non-commercial use. Building a product on the API is fine; republishing the recipe corpus is not. Source: [github.com/Racion-App/racion](https://github.com/Racion-App/racion), AGPL-3.0.
