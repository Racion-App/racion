// Офлайн-очередь отметок «куплено»: без сети отметка ложится в localStorage и уходит на сервер,
// когда сеть вернётся (событие online или следующий запуск). Порядок сохраняется, дубли по позиции схлопываются.
import { api } from "./api";

const KEY = "racion.pending.checks";
type Pending = { planId: string; itemId: string; checked: boolean; name?: string; qty?: string; cost?: number };

function read(): Pending[] {
  try {
    return JSON.parse(localStorage.getItem(KEY) || "[]") as Pending[];
  } catch {
    return [];
  }
}
function write(list: Pending[]) {
  try {
    localStorage.setItem(KEY, JSON.stringify(list));
  } catch {
    // приватный режим
  }
}

export function queueCheck(p: Pending) {
  const list = read().filter((x) => !(x.planId === p.planId && x.itemId === p.itemId));
  list.push(p);
  write(list);
}

export function pendingCount(): number {
  return read().length;
}

let flushing = false;
export async function flushChecks(): Promise<number> {
  if (flushing || !navigator.onLine) return 0;
  flushing = true;
  let sent = 0;
  try {
    const list = read();
    const rest: Pending[] = [];
    for (const p of list) {
      try {
        await api.setCheck(p.planId, { itemId: p.itemId, checked: p.checked, name: p.name, qty: p.qty, cost: p.cost });
        sent++;
      } catch {
        rest.push(p);
      }
    }
    write(rest);
  } finally {
    flushing = false;
  }
  return sent;
}

export function initOffline() {
  window.addEventListener("online", () => void flushChecks());
  void flushChecks();
}
