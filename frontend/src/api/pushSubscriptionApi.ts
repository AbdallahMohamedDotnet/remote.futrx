import { pushApi } from "./pushApi";
import { pushServiceWorkerApi } from "./pushServiceWorkerApi";
import type { PushBlocker, PushSubscriptionPayload } from "../models/push";
import { webPushTransport } from "../transport/webPushTransport";
import {
  reconcileSubscriptionOwnership,
  revokeSubscriptionForLogout,
} from "./pushSubscriptionOwnership";

type PushServerApi = Pick<
  typeof pushApi,
  "subscribe" | "unsubscribe" | "subscriptionStatus"
>;
type PushWorkerApi = Pick<
  typeof pushServiceWorkerApi,
  "isSupported" | "register" | "ready" | "currentRegistration"
>;
type PushBrowserTransport = Pick<
  typeof webPushTransport,
  | "isPushManagerSupported"
  | "isNotificationSupported"
  | "notificationPermission"
  | "requestPermission"
  | "currentSubscription"
  | "subscribe"
  | "unsubscribe"
  | "matchesApplicationServerKey"
  | "isIOS"
  | "isStandalone"
>;

export class PushSubscriptionApi {
  private readonly server: PushServerApi;
  private readonly worker: PushWorkerApi;
  private readonly browser: PushBrowserTransport;

  constructor(
    server: PushServerApi = pushApi,
    worker: PushWorkerApi = pushServiceWorkerApi,
    browser: PushBrowserTransport = webPushTransport
  ) {
    this.server = server;
    this.worker = worker;
    this.browser = browser;
  }

  /** Why this browser cannot subscribe, or null when it can. */
  blocker(serverEnabled: boolean): PushBlocker | null {
    if (!serverEnabled) return "server-disabled";
    if (!this.worker.isSupported || !this.browser.isPushManagerSupported) {
      // Safari only exposes PushManager to installed web apps, so an iPhone in
      // the browser lands here and needs the home-screen hint.
      return this.browser.isIOS() && !this.browser.isStandalone()
        ? "install-required"
        : "unsupported";
    }
    if (!this.browser.isNotificationSupported) return "unsupported";
    if (this.browser.notificationPermission === "denied") return "denied";
    return null;
  }

  /**
   * Removes an origin-wide browser subscription unless the server confirms
   * that its exact endpoint belongs to the signed-in account. This runs on
   * every authenticated app boot, not only when Settings happens to open.
   */
  async reconcileCurrentAccount(): Promise<boolean> {
    const subscription = await this.#currentSubscription();
    if (!subscription) return false;

    return reconcileSubscriptionOwnership(
      subscription,
      async (endpoint) => (await this.server.subscriptionStatus(endpoint)).owned,
      (candidate) => this.browser.unsubscribe(candidate)
    );
  }

  /**
   * Asks for permission, subscribes this device, and registers it with the
   * server. Must be called from a user gesture — iOS rejects it otherwise.
   */
  async enable(publicKey: string): Promise<void> {
    // Ask first, before any await. Safari ties requestPermission to the user
    // activation that triggered it, and awaiting anything first spends it.
    const permission = await this.browser.requestPermission();
    if (permission !== "granted") {
      throw new Error(
        permission === "denied"
          ? "Notifications are blocked for this site. Allow them in your browser settings, then try again."
          : "Notification permission was dismissed."
      );
    }

    const registration = await this.worker.register();
    if (!registration) throw new Error("This browser cannot register a service worker.");
    // Registration only queues the install; PushManager needs an active worker.
    await this.worker.ready();

    const existing = await this.browser.currentSubscription(registration);
    // A stored subscription signed with a previous server key can never be
    // delivered to, so replace it rather than reporting success.
    if (existing && !this.browser.matchesApplicationServerKey(existing, publicKey)) {
      await this.browser.unsubscribe(existing);
    }

    const subscription =
      (await this.browser.currentSubscription(registration)) ??
      (await this.browser.subscribe(registration, publicKey));

    await this.server.subscribe(this.#payload(subscription));
  }

  /** Removes this device, both locally and on the server. */
  async disable(): Promise<void> {
    const subscription = await this.#currentSubscription();
    if (!subscription) return;
    // Tell the server first: if unsubscribing locally succeeded but the server
    // kept the endpoint, it would keep pushing to a dead registration.
    await this.server.unsubscribe(subscription.endpoint);
    await this.browser.unsubscribe(subscription);
  }

  /** Revokes this browser on both sides before its session cookie is cleared. */
  async prepareForLogout(): Promise<void> {
    const subscription = await this.#currentSubscription();
    if (!subscription) return;

    await revokeSubscriptionForLogout(
      subscription,
      (endpoint) => this.server.unsubscribe(endpoint),
      (candidate) => this.browser.unsubscribe(candidate)
    );
  }

  #payload(subscription: PushSubscription): PushSubscriptionPayload {
    // PushManager-created subscriptions contain the endpoint and both keys.
    // Keep the browser's serialized object intact at the server boundary.
    return subscription.toJSON() as PushSubscriptionPayload;
  }

  async #currentSubscription(): Promise<PushSubscription | null> {
    const registration = await this.worker.currentRegistration();
    if (!registration) return null;
    return this.browser.currentSubscription(registration);
  }
}

export const pushSubscriptionApi = new PushSubscriptionApi();
