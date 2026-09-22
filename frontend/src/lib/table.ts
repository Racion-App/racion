import { readJSON, writeJSON } from "./storage";

// «Мой стол» — блюда, которые человек выбрал сам, чтобы получить один список покупок.
// Хранилище общее с серверными страницами: там кнопки ставит public/table.js, читает и пишет тот же ключ.

export const TABLE_KEY = "racion.table";
export const TABLE_MAX = 30;

// servings = 0 значит «как у стола»: сколько человек за столом, столько и порций.
export type TableItem = { id: string; servings: number };

export function readTable(): TableItem[] {
  const raw = readJSON<unknown>(TABLE_KEY, []);
  if (!Array.isArray(raw)) return [];
  return raw
    .filter((x): x is TableItem => !!x && typeof (x as TableItem).id === "string")
    .slice(0, TABLE_MAX)
    .map((x) => ({ id: x.id, servings: Number.isFinite(x.servings) ? Math.max(0, Math.min(40, Math.round(x.servings))) : 0 }));
}

export function writeTable(list: TableItem[]) {
  writeJSON(TABLE_KEY, list.slice(0, TABLE_MAX));
}

export function inTable(list: TableItem[], id: string): boolean {
  return list.some((x) => x.id === id);
}

export function toggleTable(id: string): TableItem[] {
  const list = readTable();
  const next = inTable(list, id) ? list.filter((x) => x.id !== id) : list.length < TABLE_MAX ? [...list, { id, servings: 0 }] : list;
  writeTable(next);
  return next;
}

export function setServings(id: string, servings: number): TableItem[] {
  const next = readTable().map((x) => (x.id === id ? { ...x, servings: Math.max(0, Math.min(40, Math.round(servings))) } : x));
  writeTable(next);
  return next;
}

export function clearTable() {
  writeTable([]);
}
