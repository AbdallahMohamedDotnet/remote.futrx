import { pushApi } from "../../api/pushApi";
import type { PushBlocker } from "../../models/push";
import { registerServiceWorker, serviceWorkerSupported } from "./serviceWorkerClient";

/**
 * Why this browser cannot subscribe, or null when it can. Checked before the
 * permission prompt so the UI can explain rather than fail silently.
 */
export function pushBlocker(serverEnabled: boolean): PushBlocker | null {
  if (!serverEnabled) return "server-disabled";
  if (!serviceWorkerSupported() || !("PushManager" in window)) {
    // Safari only exposes PushManager to installed web apps, so an iPhone in
    // the browser lands here and needs the home-screen hint, not "your
    // browser is too old".
    return isIOS() && !isStandalone() ? "install-required" : "unsupported";
  }
  if (!("Notification" in window)) return "unsupported";
  if (Notification.permission === "denied") return "denied";
  return null;
}

/** Whether this account already receives notifications on this device. */
export async function currentSubscription(): Promise<PushSubscription | null> {
  if (!serviceWorkerSupported()) return null;
  const registration = await navigator.serviceWorker.getRegistration("/");
  if (!registration) return null;
  return registration.pushManager.getSubscription();
}

/**
 * Asks for permission, subscribes this device, and registers it with the
 * server. Must be called from a user gesture — iOS rejects it otherwise.
 */
export async function enablePush(publicKey: string): Promise<void> {
  // Ask first, before any await. Safari ties requestPermission to the user
  // activation that triggered it, and awaiting anything first spends it.
  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    throw new Error(
      permission === "denied"
        ? "Notifications are blocked for this site. Allow them in your browser settings, then try again."
        : "Notification permission was dismissed."
    );
  }

  const registration = await registerServiceWorker();
  if (!registration) throw new Error("This browser cannot register a service worker.");
  // Registration only queues the install; pushManager needs an active worker.
  await navigator.serviceWorker.ready;

  const existing = await registration.pushManager.getSubscription();
  // A stored subscription signed with a previous server key can never be
  // delivered to, so replace it rather than reporting success.
  if (existing && !matchesServerKey(existing, publicKey)) {
    await existing.unsubscribe();
  }

  const subscription =
    (await registration.pushManager.getSubscription()) ??
    (await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: decodeBase64Url(publicKey),
    }));

  await pushApi.subscribe(subscription.toJSON() as never);
}

/** Removes this device, both locally and on the server. */
export async function disablePush(): Promise<void> {
  const subscription = await currentSubscription();
  if (!subscription) return;
  // Tell the server first: if unsubscribing locally succeeded but the server
  // kept the endpoint, it would keep pushing to a dead registration.
  await pushApi.unsubscribe(subscription.endpoint);
  await subscription.unsubscribe();
}

function matchesServerKey(subscription: PushSubscription, publicKey: string): boolean {
  const applied = subscription.options?.applicationServerKey;
  if (!applied) return false;
  return encodeBase64Url(new Uint8Array(applied)) === publicKey.replace(/=+$/, "");
}

/** VAPID keys travel as base64url; PushManager wants the raw bytes. */
function decodeBase64Url(value: string): Uint8Array<ArrayBuffer> {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(padded.padEnd(padded.length + ((4 - (padded.length % 4)) % 4), "="));
  const bytes = new Uint8Array(new ArrayBuffer(binary.length));
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

function encodeBase64Url(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function isIOS(): boolean {
  return (
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    // iPadOS reports itself as a Mac, but is the only "Mac" with touch.
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1)
  );
}

function isStandalone(): boolean {
  return (
    window.matchMedia("(display-mode: standalone)").matches ||
    (window.navigator as { standalone?: boolean }).standalone === true
  );
}
