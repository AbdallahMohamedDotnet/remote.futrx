import { requestJson } from "./apiRequest";
import { subscribeToJsonMessages } from "../transport/jsonMessageSubscription";
import type { ApplicationPath } from "../types/transport";

interface DeviceAuthRoutes {
  readonly status: ApplicationPath;
  readonly startDeviceLogin: ApplicationPath;
  readonly statusUpdates: ApplicationPath;
}

export class DeviceAuthApi<TStatus, TDeviceLogin> {
  readonly #routes: DeviceAuthRoutes;

  constructor(routes: DeviceAuthRoutes) {
    this.#routes = routes;
  }

  readonly fetchStatus = (): Promise<TStatus> =>
    requestJson<TStatus>("GET", this.#routes.status);

  readonly startDeviceLogin = (): Promise<TDeviceLogin> =>
    requestJson<TDeviceLogin>("POST", this.#routes.startDeviceLogin, {});

  readonly subscribe = (
    onStatus: (status: TStatus) => void
  ): (() => void) =>
    subscribeToJsonMessages(this.#routes.statusUpdates, onStatus);
}
