export class ServiceWorkerTransport {
  get isSupported(): boolean {
    return typeof navigator !== "undefined" && "serviceWorker" in navigator;
  }

  register(scriptUrl: string, options: RegistrationOptions): Promise<ServiceWorkerRegistration> {
    return navigator.serviceWorker.register(scriptUrl, options);
  }

  listen(listener: (event: MessageEvent) => void): void {
    navigator.serviceWorker.addEventListener("message", listener);
  }
}

export const serviceWorkerTransport = new ServiceWorkerTransport();
