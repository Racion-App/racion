import { api } from "./api";
import { readJSON, writeJSON } from "./storage";

// Web Push на этом устройстве: регистрация service worker, подписка через VAPID-ключ сервера,
// отправка подписки в аккаунт. Состояние: unsupported | denied | off | on.

export type PushState = "unsupported" | "denied" | "off" | "on";

export function pushSupported(): boolean {
  return typeof window !== "undefined" && "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
}

async function registration(): Promise<ServiceWorkerRegistration> {
  const reg = await navigator.serviceWorker.register("/sw.js", { scope: "/" });
  await navigator.serviceWorker.ready;
  return reg;
}

export async function pushState(): Promise<PushState> {
  if (!pushSupported()) return "unsupported";
  if (Notification.permission === "denied") return "denied";
  try {
    const reg = await navigator.serviceWorker.getRegistration("/");
    const sub = await reg?.pushManager.getSubscription();
    return sub ? "on" : "off";
  } catch {
    return "off";
  }
}

function sameKey(a: ArrayBuffer | null | undefined, b: Uint8Array): boolean {
  if (!a) return false;
  const x = new Uint8Array(a);
  if (x.length !== b.length) return false;
  for (let i = 0; i < x.length; i++) if (x[i] !== b[i]) return false;
  return true;
}

function toKey(base64: string): Uint8Array {
  const pad = "=".repeat((4 - (base64.length % 4)) % 4);
  const raw = atob((base64 + pad).replace(/-/g, "+").replace(/_/g, "/"));
  return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

type Keys = { endpoint: string; p256dh: string; auth: string };

// browserSubscription запрашивает разрешение и подписывает браузер на ключ сервера; строка — почему не вышло.
async function browserSubscription(): Promise<Keys | PushState> {
  if (!pushSupported()) return "unsupported";
  const perm = await Notification.requestPermission();
  if (perm !== "granted") return perm === "denied" ? "denied" : "off";
  const { key } = await api.pushKey();
  const reg = await registration();
  const wanted = toKey(key);
  let sub = await reg.pushManager.getSubscription();
  // подписка от другого сервера (старые ключи VAPID): push-сервис ответит 403, поэтому переподписываемся
  if (sub && !sameKey(sub.options.applicationServerKey, wanted)) {
    await sub.unsubscribe().catch(() => undefined);
    sub = null;
  }
  if (!sub) sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: wanted as BufferSource });
  const j = sub.toJSON();
  return { endpoint: sub.endpoint, p256dh: j.keys?.p256dh ?? "", auth: j.keys?.auth ?? "" };
}

// subscribe запрашивает разрешение и регистрирует устройство в аккаунте.
export async function pushSubscribe(): Promise<PushState> {
  const sub = await browserSubscription();
  if (typeof sub === "string") return sub;
  await api.pushSubscribe(sub);
  return "on";
}

// Без аккаунта: устройство подписывается на напоминания по неделе. Выбор человека (подписано ли
// устройство, нужно ли утреннее «что готовим») помнит браузер — с ним подписка уходит и для каждой
// новой недели.
const DEVICE_KEY = "racion.remind";
type DeviceRemind = { on: boolean; today: boolean };

export function deviceRemind(): DeviceRemind {
  return readJSON<DeviceRemind>(DEVICE_KEY, { on: false, today: true });
}

export async function pushSubscribeDevice(planId: string, today: boolean): Promise<PushState> {
  const sub = await browserSubscription();
  if (typeof sub === "string") return sub;
  await api.pushDevice({ ...sub, planId, today, tz: -new Date().getTimezoneOffset() });
  writeJSON(DEVICE_KEY, { on: true, today });
  return "on";
}

// pushLinkDevice — открыли другую неделю на подписанном устройстве: напоминания пойдут и по ней.
// Без запроса разрешения: если подписки в браузере уже нет, просто забываем о ней.
export async function pushLinkDevice(planId: string): Promise<void> {
  const r = deviceRemind();
  if (!r.on || !pushSupported() || Notification.permission !== "granted") return;
  const reg = await navigator.serviceWorker.getRegistration("/");
  const sub = await reg?.pushManager.getSubscription();
  if (!sub) {
    writeJSON(DEVICE_KEY, { ...r, on: false });
    return;
  }
  const j = sub.toJSON();
  await api.pushDevice({ endpoint: sub.endpoint, p256dh: j.keys?.p256dh ?? "", auth: j.keys?.auth ?? "", planId, today: r.today, tz: -new Date().getTimezoneOffset() });
}

export async function pushUnsubscribeDevice(): Promise<PushState> {
  const reg = await navigator.serviceWorker.getRegistration("/");
  const sub = await reg?.pushManager.getSubscription();
  if (sub) {
    await api.pushDeviceDelete(sub.endpoint).catch(() => undefined);
    await sub.unsubscribe();
  }
  writeJSON(DEVICE_KEY, { ...deviceRemind(), on: false });
  return "off";
}

export async function pushUnsubscribe(): Promise<PushState> {
  const reg = await navigator.serviceWorker.getRegistration("/");
  const sub = await reg?.pushManager.getSubscription();
  if (sub) {
    await api.pushUnsubscribe(sub.endpoint).catch(() => undefined);
    await sub.unsubscribe();
  }
  return "off";
}

// pushTestResult ждёт проверочный пуш после запроса к серверу и возвращает ключ перевода с итогом:
// уведомление показано; дошло до браузера, но система его не вывела; не дошло вовсе.
export async function pushTestResult(): Promise<string> {
  const reg = await navigator.serviceWorker.getRegistration("/");
  if (!reg) return "notify.test.sent";
  let arrived = false;
  const onMsg = (e: MessageEvent) => { if (e.data?.type === "push" && e.data.tag === "test") arrived = true; };
  navigator.serviceWorker.addEventListener("message", onMsg);
  try {
    for (let i = 0; i < 16; i++) {
      await new Promise((r) => setTimeout(r, 500));
      const shown = await reg.getNotifications({ tag: "test" }).catch(() => []);
      if (shown.length) return "notify.test.sent";
      // пуш пришёл, showNotification отработал, а в списке пусто: система (Windows, iOS) его не показала
      if (arrived && i >= 5) return "notify.test.hidden";
    }
    return arrived ? "notify.test.hidden" : "notify.test.lost";
  } finally {
    navigator.serviceWorker.removeEventListener("message", onMsg);
  }
}
