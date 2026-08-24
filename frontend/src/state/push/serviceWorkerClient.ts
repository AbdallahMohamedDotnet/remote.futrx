import { pushServiceWorkerApi } from "../../api/pushServiceWorkerApi";

export function serviceWorkerSupported(): boolean {
  return pushServiceWorkerApi.isSupported;
}

/**
 * Registers the worker and returns its registration. Resolves to null when the
 * browser has no service worker support, so callers can degrade quietly.
 */
export async function registerServiceWorker(): Promise<ServiceWorkerRegistration | null> {
  return pushServiceWorkerApi.register();
}
