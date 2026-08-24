import { serviceWorkerTransport } from "../transport/serviceWorkerTransport";

const SERVICE_WORKER_URL = "/sw.js";

interface PushServiceWorkerCallbacks {
  visibleChatId: () => string | null;
  openChat: (chatId: string | null) => void;
}

class PushServiceWorkerApi {
  get isSupported(): boolean {
    return serviceWorkerTransport.isSupported;
  }

  async register(): Promise<ServiceWorkerRegistration | null> {
    if (!this.isSupported) return null;
    try {
      return await serviceWorkerTransport.register(SERVICE_WORKER_URL, { scope: "/" });
    } catch {
      return null;
    }
  }

  listen(callbacks: PushServiceWorkerCallbacks): void {
    serviceWorkerTransport.listen((event) => {
      const message = event.data;
      if (!message || typeof message !== "object") return;

      if (message.type === "which-chat") {
        event.ports[0]?.postMessage({ chatId: callbacks.visibleChatId() });
        return;
      }

      if (message.type === "open-chat") {
        callbacks.openChat(message.chatId ?? null);
      }
    });
  }
}

export const pushServiceWorkerApi = new PushServiceWorkerApi();
