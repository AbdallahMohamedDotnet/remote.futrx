import { pushSubscriptionApi } from "../../api/pushSubscriptionApi";
import type { PushBlocker } from "../../models/push";

/**
 * Why this browser cannot subscribe, or null when it can. Checked before the
 * permission prompt so the UI can explain rather than fail silently.
 */
export function pushBlocker(serverEnabled: boolean): PushBlocker | null {
  return pushSubscriptionApi.blocker(serverEnabled);
}

/** Whether this account already receives notifications on this device. */
export function currentSubscription(): Promise<PushSubscription | null> {
  return pushSubscriptionApi.currentSubscription();
}

/**
 * Asks for permission, subscribes this device, and registers it with the
 * server. Must be called from a user gesture — iOS rejects it otherwise.
 */
export function enablePush(publicKey: string): Promise<void> {
  return pushSubscriptionApi.enable(publicKey);
}

/** Removes this device, both locally and on the server. */
export function disablePush(): Promise<void> {
  return pushSubscriptionApi.disable();
}
