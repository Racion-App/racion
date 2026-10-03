import type { Plan } from "./types";
import { readJSON, writeJSON } from "./storage";

// Имена едоков гостевого плана живут только в браузере автора. По ссылке сервер отдаёт план без имён:
// «Маша, похудение, 1500 ккал» не должно уходить чужим людям. Своей семье (владельцу и присоединившимся)
// сервер имена отдаёт сам, а автору без входа подставляем их отсюда.
const key = (id: string) => `racion.names.${id}`;

export function rememberNames(plan: Plan) {
  const names = (plan.members ?? []).map((m) => m.name ?? "");
  if (names.some(Boolean)) writeJSON(key(plan.id), names);
}

export function withNames(plan: Plan): Plan {
  const members = plan.members;
  if (!members?.length || members.some((m) => m.name)) return plan;
  const names = readJSON<string[]>(key(plan.id), []);
  if (!names.some(Boolean)) return plan;
  return { ...plan, members: members.map((m, i) => ({ ...m, name: names[i] ?? "" })) };
}
