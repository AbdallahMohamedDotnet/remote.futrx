// Service worker for remote.futrx.
//
// Its main job is Web Push: showing a notification when an agent asks a
// question, finishes a turn, fails, or a scheduled task runs. There is
// deliberately no app-shell caching — this app is a live control plane for
// running agents, and a stale cached shell would be worse than an honest
// network error. The one exception is a static offline page, shown only when
// a navigation cannot reach the network at all.

const ICON = "/icon-192.png";
const BADGE = "/badge-96.png";

// Bump the version when offline.html changes so installed clients refresh it.
const OFFLINE_CACHE = "offline-v1";
const OFFLINE_URL = "/offline.html";

// How long to wait for an open tab to say which chat it is showing before
// deciding the notification is worth raising anyway.
const CLIENT_REPLY_TIMEOUT_MS = 400;

self.addEventListener("install", (event) => {
  // Take over immediately: a stale worker would keep using the old push
  // payload shape after a deploy.
  self.skipWaiting();
  event.waitUntil(
    caches.open(OFFLINE_CACHE).then((cache) =>
      // "reload" bypasses the HTTP cache so a deploy always ships the page.
      cache.add(new Request(OFFLINE_URL, { cache: "reload" }))
    )
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(keys.filter((key) => key !== OFFLINE_CACHE).map((key) => caches.delete(key)));
      await self.clients.claim();
    })()
  );
});

// Navigations go to the network as always; the offline page appears only when
// the network is unreachable. Every other request falls through untouched.
self.addEventListener("fetch", (event) => {
  if (event.request.mode !== "navigate") return;
  event.respondWith(
    fetch(event.request).catch(async () => {
      const cached = await caches.match(OFFLINE_URL);
      return cached || Response.error();
    })
  );
});

self.addEventListener("push", (event) => {
  event.waitUntil(handlePush(event));
});

async function handlePush(event) {
  const payload = readPayload(event);
  if (!payload) return;

  // Don't buzz a phone the user is already staring at the chat on.
  if (payload.chatId && (await isChatOnScreen(payload.chatId))) return;

  const urgent = payload.kind === "question";
  await self.registration.showNotification(payload.title || "remote.futrx", {
    body: payload.body || "",
    tag: payload.tag || "remote-futrx",
    icon: ICON,
    badge: BADGE,
    data: { chatId: payload.chatId || null, kind: payload.kind || null },
    // A parked run stays on screen until it is acknowledged; routine
    // completions fade like any other notification.
    requireInteraction: urgent,
    renotify: Boolean(payload.tag),
    vibrate: urgent ? [90, 60, 90] : undefined,
    timestamp: Date.now(),
  });
}

function readPayload(event) {
  if (!event.data) return null;
  try {
    return event.data.json();
  } catch {
    const text = event.data.text();
    return text ? { title: text } : null;
  }
}

// Ask every visible window which chat it is showing. Asking beats tracking
// state in the worker, which the browser is free to evict between pushes.
async function isChatOnScreen(chatId) {
  const windows = await self.clients.matchAll({
    type: "window",
    includeUncontrolled: true,
  });
  const visible = windows.filter((client) => client.visibilityState === "visible");
  if (visible.length === 0) return false;

  const answers = await Promise.all(visible.map((client) => askClient(client, { type: "which-chat" })));
  return answers.some((answer) => answer && answer.chatId === chatId);
}

function askClient(client, message) {
  return new Promise((resolve) => {
    const channel = new MessageChannel();
    const timer = setTimeout(() => resolve(null), CLIENT_REPLY_TIMEOUT_MS);
    channel.port1.onmessage = (event) => {
      clearTimeout(timer);
      resolve(event.data);
    };
    try {
      client.postMessage(message, [channel.port2]);
    } catch {
      clearTimeout(timer);
      resolve(null);
    }
  });
}

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  event.waitUntil(openChat(event.notification.data && event.notification.data.chatId));
});

async function openChat(chatId) {
  const windows = await self.clients.matchAll({
    type: "window",
    includeUncontrolled: true,
  });
  for (const client of windows) {
    if (new URL(client.url).origin !== self.location.origin) continue;
    // Focus first: some browsers ignore a postMessage-driven view change in
    // a window that never came forward.
    if ("focus" in client) await client.focus();
    client.postMessage({ type: "open-chat", chatId: chatId || null });
    return;
  }
  // Cold start: the app reads ?chat= on boot and opens straight into it.
  await self.clients.openWindow(chatId ? `/?chat=${encodeURIComponent(chatId)}` : "/");
}
