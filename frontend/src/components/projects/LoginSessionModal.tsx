// LoginSessionModal: captures a single login flow inside a Chromium that
// runs inside the project's LXD container. Drawn into a <canvas>; mouse +
// keyboard events are forwarded over WS to the backend, which relays them
// to Chromium via CDP. The "Done capturing" button (wired in milestone 3)
// will POST a capture endpoint that writes a project secret.

import { useEffect, useRef, useState } from "preact/hooks";
import type { JSX } from "preact";
import { loginSessionService } from "../../services/loginSessionService";
import type { LoginSessionStart, CaptureResult } from "../../services/loginSessionService";
import { Loader, X, RotateCcw, Check } from "../ui/icons";

const NATIVE_WIDTH = 1280;
const NATIVE_HEIGHT = 720;

type Phase = "form" | "connecting" | "live" | "capturing" | "captured" | "error";

export function LoginSessionModal({
  projectId,
  open,
  initialUrl,
  initialName,
  onClose,
  onCaptured,
}: {
  projectId: string;
  open: boolean;
  initialUrl?: string;
  initialName?: string;
  onClose: () => void;
  onCaptured?: (result: CaptureResult) => void;
}) {
  const [url, setUrl] = useState(initialUrl ?? "");
  const [name, setName] = useState(initialName ?? "");
  const [phase, setPhase] = useState<Phase>("form");
  const [error, setError] = useState<string | null>(null);
  const [statusText, setStatusText] = useState<string>("");
  const [captureSummary, setCaptureSummary] = useState<CaptureResult | null>(null);
  const [currentUrl, setCurrentUrl] = useState<string>("");

  const session = useRef<LoginSessionStart | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const overlayRef = useRef<HTMLDivElement | null>(null);
  const cleanupRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    if (!open) {
      cleanupRef.current();
      cleanupRef.current = () => undefined;
      setPhase("form");
      setError(null);
      setStatusText("");
      setCaptureSummary(null);
      setCurrentUrl("");
      session.current = null;
    } else {
      setUrl(initialUrl ?? "");
      setName(initialName ?? "");
    }
  }, [open, initialUrl, initialName]);

  // Tear down WS + tell backend to stop on unmount or close.
  useEffect(() => () => cleanupRef.current(), []);

  const start = async (e: JSX.TargetedSubmitEvent<HTMLFormElement>) => {
    e.preventDefault();
    const trimmedUrl = url.trim();
    const trimmedName = name.trim();
    if (!trimmedUrl || !trimmedName) {
      setError("URL and name are required");
      return;
    }
    setError(null);
    setPhase("connecting");
    setStatusText("Starting Chromium…");
    try {
      const sess = await loginSessionService.start(projectId, trimmedUrl, trimmedName);
      session.current = sess;
      setCurrentUrl(sess.url);
      openSocket(sess);
    } catch (err) {
      setError((err as Error).message);
      setPhase("error");
    }
  };

  const openSocket = (sess: LoginSessionStart) => {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}${sess.wsPath}`);
    wsRef.current = ws;
    setStatusText("Connecting to browser…");

    ws.onmessage = (evt) => {
      let msg: { type: string; [k: string]: unknown };
      try {
        msg = JSON.parse(evt.data);
      } catch {
        return;
      }
      switch (msg.type) {
        case "ready":
          setPhase("live");
          setStatusText("Live");
          break;
        case "frame":
          drawFrame(msg.data as string);
          break;
        case "url":
          if (typeof msg.url === "string") setCurrentUrl(msg.url);
          break;
        case "error":
          setError(String(msg.message ?? "unknown error"));
          setPhase("error");
          break;
        case "warn":
          // surface but don't fail
          setStatusText(String(msg.message ?? ""));
          break;
      }
    };
    ws.onerror = () => {
      setError("WebSocket error");
      setPhase("error");
    };
    ws.onclose = () => {
      if (phase === "live") setStatusText("Disconnected");
    };

    cleanupRef.current = () => {
      try { ws.close(); } catch { /* ignore */ }
      const s = session.current;
      session.current = null;
      if (s) {
        void loginSessionService.stop(projectId, s.id).catch(() => undefined);
      }
    };
  };

  const drawFrame = (b64: string) => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const img = new Image();
    img.onload = () => {
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      if (canvas.width !== img.width || canvas.height !== img.height) {
        canvas.width = img.width;
        canvas.height = img.height;
      }
      ctx.drawImage(img, 0, 0);
    };
    img.src = `data:image/jpeg;base64,${b64}`;
  };

  // ---------- Input forwarding ----------

  const sendInput = (payload: Record<string, unknown>) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify(payload));
  };

  // Translate a DOM event coordinate (from overlay div) into native-canvas
  // pixels so CDP receives coordinates that match what Chromium rendered.
  const eventCoords = (e: { clientX: number; clientY: number }) => {
    const overlay = overlayRef.current;
    if (!overlay) return { x: 0, y: 0 };
    const rect = overlay.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * NATIVE_WIDTH;
    const y = ((e.clientY - rect.top) / rect.height) * NATIVE_HEIGHT;
    return { x: Math.max(0, Math.min(NATIVE_WIDTH, x)), y: Math.max(0, Math.min(NATIVE_HEIGHT, y)) };
  };

  const buttonName = (b: number): string => {
    switch (b) {
      case 0: return "left";
      case 1: return "middle";
      case 2: return "right";
      case 3: return "back";
      case 4: return "forward";
      default: return "left";
    }
  };

  const modifiersFromEvent = (e: { altKey?: boolean; ctrlKey?: boolean; metaKey?: boolean; shiftKey?: boolean }): number => {
    let m = 0;
    if (e.altKey) m |= 1;
    if (e.ctrlKey) m |= 2;
    if (e.metaKey) m |= 4;
    if (e.shiftKey) m |= 8;
    return m;
  };

  const onPointerMove = (e: PointerEvent) => {
    if (phase !== "live") return;
    const { x, y } = eventCoords(e);
    sendInput({ type: "move", x, y, modifiers: modifiersFromEvent(e) });
  };

  const onPointerDown = (e: PointerEvent) => {
    if (phase !== "live") return;
    e.preventDefault();
    const overlay = overlayRef.current;
    if (overlay) overlay.focus();
    const { x, y } = eventCoords(e);
    sendInput({
      type: "down",
      x, y,
      button: buttonName(e.button),
      clickCount: 1,
      modifiers: modifiersFromEvent(e),
    });
  };

  const onPointerUp = (e: PointerEvent) => {
    if (phase !== "live") return;
    e.preventDefault();
    const { x, y } = eventCoords(e);
    sendInput({
      type: "up",
      x, y,
      button: buttonName(e.button),
      clickCount: 1,
      modifiers: modifiersFromEvent(e),
    });
  };

  const onWheel = (e: WheelEvent) => {
    if (phase !== "live") return;
    e.preventDefault();
    const { x, y } = eventCoords(e);
    sendInput({ type: "scroll", x, y, dx: e.deltaX, dy: e.deltaY, modifiers: modifiersFromEvent(e) });
  };

  const onContextMenu = (e: MouseEvent) => {
    // Don't pop the native menu when right-clicking the captured browser.
    e.preventDefault();
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (phase !== "live") return;
    // Allow Cmd+W / Ctrl+W / Esc to close on the OUR side.
    if (e.key === "Escape" && (e.metaKey || e.ctrlKey)) {
      onClose();
      return;
    }
    e.preventDefault();
    const mod = modifiersFromEvent(e);
    sendInput({
      type: "key",
      key: e.key,
      code: e.code,
      keyCode: (e as KeyboardEvent & { keyCode?: number }).keyCode ?? 0,
      modifiers: mod,
      location: e.location ?? 0,
      isKeyDown: true,
      text: e.key.length === 1 && !e.ctrlKey && !e.metaKey ? e.key : "",
    });
  };

  const onKeyUp = (e: KeyboardEvent) => {
    if (phase !== "live") return;
    e.preventDefault();
    sendInput({
      type: "key",
      key: e.key,
      code: e.code,
      keyCode: (e as KeyboardEvent & { keyCode?: number }).keyCode ?? 0,
      modifiers: modifiersFromEvent(e),
      location: e.location ?? 0,
      isKeyDown: false,
    });
  };

  const reload = () => sendInput({ type: "reload" });
  const goBack = () => sendInput({ type: "back" });
  const goForward = () => sendInput({ type: "forward" });

  const navigate = (e: JSX.TargetedSubmitEvent<HTMLFormElement>) => {
    e.preventDefault();
    const u = currentUrl.trim();
    if (!u) return;
    sendInput({ type: "navigate", url: u });
  };

  const cancel = () => {
    cleanupRef.current();
    cleanupRef.current = () => undefined;
    onClose();
  };

  const finish = async () => {
    const s = session.current;
    if (!s) return;
    setPhase("capturing");
    setStatusText("Capturing storage state…");
    try {
      const result = await loginSessionService.capture(projectId, s.id);
      setCaptureSummary(result);
      setPhase("captured");
      setStatusText(`Saved ${result.secretName}`);
      // The capture endpoint also stops the session backend-side; clear
      // local cleanup so the close path doesn't try to stop again.
      session.current = null;
      cleanupRef.current = () => undefined;
      try { wsRef.current?.close(); } catch { /* ignore */ }
      onCaptured?.(result);
    } catch (err) {
      setError((err as Error).message);
      setPhase("error");
    }
  };

  if (!open) return null;

  return (
    <div
      class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm grid place-items-center p-4"
      aria-modal="true"
      role="dialog"
    >
      <div class="bg-[#101318] border border-white/10 rounded-lg shadow-2xl w-full max-w-5xl flex flex-col max-h-[90vh] min-h-[400px]">
        <header class="flex items-center gap-2 px-3 py-2 border-b border-white/10">
          <div class="flex-1 min-w-0">
            <div class="text-[11px] text-ink-300">Login session</div>
            <div class="text-[14px] text-ink-50 font-medium truncate">
              {name || initialName || "New login"}
              {session.current?.secretName && (
                <span class="text-ink-300 font-normal ml-2 text-[12px]">→ {session.current.secretName}</span>
              )}
            </div>
          </div>
          <div class="text-[11px] text-ink-300 mr-2 hidden sm:block">{statusText}</div>
          <button
            type="button"
            onClick={cancel}
            class="h-8 w-8 rounded text-ink-300 hover:text-ink-50 hover:bg-white/10 grid place-items-center"
            aria-label="Close"
          >
            <X class="w-4 h-4" />
          </button>
        </header>

        {phase === "form" && (
          <form onSubmit={start} class="p-4 space-y-3 flex-1">
            <div>
              <label class="block text-[12px] text-ink-300 mb-1">URL</label>
              <input
                type="url"
                required
                value={url}
                onInput={(e) => setUrl((e.target as HTMLInputElement).value)}
                placeholder="https://example.com/login"
                class="w-full bg-[#0b0d11] border border-white/10 rounded px-2 py-1.5 text-sm text-ink-50"
              />
            </div>
            <div>
              <label class="block text-[12px] text-ink-300 mb-1">Name</label>
              <input
                type="text"
                required
                value={name}
                onInput={(e) => setName((e.target as HTMLInputElement).value)}
                placeholder="my-service"
                class="w-full bg-[#0b0d11] border border-white/10 rounded px-2 py-1.5 text-sm text-ink-50"
              />
              <div class="mt-1 text-[11px] text-ink-300">
                Saved as secret <code class="text-ink-100">STORAGE_STATE_{secretSuffix(name)}</code>.
              </div>
            </div>
            {error && (
              <div class="text-[12px] text-red-300 bg-red-900/30 border border-red-700/40 rounded px-2 py-1">
                {error}
              </div>
            )}
            <div class="flex gap-2 justify-end">
              <button type="button" onClick={cancel} class="px-3 py-1.5 text-sm text-ink-200 hover:bg-white/[0.08] rounded">
                Cancel
              </button>
              <button type="submit" class="px-3 py-1.5 text-sm bg-accent-blue/80 text-white rounded hover:bg-accent-blue">
                Start
              </button>
            </div>
          </form>
        )}

        {phase !== "form" && (
          <>
            <div class="flex items-center gap-1 px-2 py-1.5 border-b border-white/10">
              <button type="button" onClick={goBack} class="h-7 w-7 rounded text-ink-200 hover:bg-white/10 grid place-items-center" title="Back">
                <ChevronLeftIcon />
              </button>
              <button type="button" onClick={goForward} class="h-7 w-7 rounded text-ink-200 hover:bg-white/10 grid place-items-center" title="Forward">
                <ChevronRightIcon />
              </button>
              <button type="button" onClick={reload} class="h-7 w-7 rounded text-ink-200 hover:bg-white/10 grid place-items-center" title="Reload">
                <RotateCcw class="w-3.5 h-3.5" />
              </button>
              <form onSubmit={navigate} class="flex-1">
                <input
                  type="text"
                  value={currentUrl}
                  onInput={(e) => setCurrentUrl((e.target as HTMLInputElement).value)}
                  class="w-full h-7 bg-[#0b0d11] border border-white/10 rounded px-2 text-[12px] text-ink-50"
                  placeholder="https://…"
                />
              </form>
            </div>

            <div class="flex-1 min-h-0 bg-black grid place-items-center overflow-hidden">
              {(phase === "connecting" || phase === "capturing") && (
                <div class="absolute z-10 text-ink-200 flex items-center gap-2 text-sm">
                  <Loader class="w-4 h-4 animate-spin" /> {statusText || "Working…"}
                </div>
              )}
              {phase === "error" && (
                <div class="text-red-300 text-sm px-4 text-center">
                  {error || "Something went wrong."}
                </div>
              )}
              {phase === "captured" && captureSummary && (
                <div class="text-ink-100 text-sm flex flex-col items-center gap-1 px-4 text-center">
                  <Check class="w-6 h-6 text-emerald-400" />
                  <div>Saved <code class="text-ink-50">{captureSummary.secretName}</code></div>
                  <div class="text-[11px] text-ink-300">
                    {captureSummary.cookieCount} cookies · {captureSummary.originCount} origins · {captureSummary.sizeBytes} bytes
                  </div>
                </div>
              )}
              <div
                ref={overlayRef}
                tabIndex={0}
                class="relative w-full h-full grid place-items-center outline-none"
                onPointerMove={onPointerMove}
                onPointerDown={onPointerDown}
                onPointerUp={onPointerUp}
                onWheel={onWheel}
                onKeyDown={onKeyDown}
                onKeyUp={onKeyUp}
                onContextMenu={onContextMenu}
              >
                <canvas
                  ref={canvasRef}
                  width={NATIVE_WIDTH}
                  height={NATIVE_HEIGHT}
                  class={`max-w-full max-h-full object-contain ${phase === "live" ? "" : "opacity-30"}`}
                />
              </div>
            </div>

            <footer class="flex items-center gap-2 justify-end px-3 py-2 border-t border-white/10">
              <button type="button" onClick={cancel} class="px-3 py-1.5 text-sm text-ink-200 hover:bg-white/[0.08] rounded">
                {phase === "captured" ? "Close" : "Cancel"}
              </button>
              {phase !== "captured" && phase !== "error" && (
                <button
                  type="button"
                  onClick={finish}
                  disabled={phase !== "live"}
                  class="px-3 py-1.5 text-sm bg-emerald-600 text-white rounded hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  Done capturing
                </button>
              )}
            </footer>
          </>
        )}
      </div>
    </div>
  );
}

function secretSuffix(name: string): string {
  let cleaned = "";
  for (const r of (name ?? "").toUpperCase()) {
    if ((r >= "A" && r <= "Z") || (r >= "0" && r <= "9")) cleaned += r;
    else if (r === "_" || r === "-" || r === " ") cleaned += "_";
  }
  while (cleaned.startsWith("_")) cleaned = cleaned.slice(1);
  while (cleaned.endsWith("_")) cleaned = cleaned.slice(0, -1);
  while (cleaned.includes("__")) cleaned = cleaned.replace("__", "_");
  if (cleaned && cleaned[0] >= "0" && cleaned[0] <= "9") cleaned = "X" + cleaned;
  return cleaned || "…";
}

// Local arrow icons (cheap inlines so we don't depend on extra icon imports).
function ChevronLeftIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
         stroke-linecap="round" stroke-linejoin="round">
      <path d="m15 18-6-6 6-6" />
    </svg>
  );
}
function ChevronRightIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
         stroke-linecap="round" stroke-linejoin="round">
      <path d="m9 6 6 6-6 6" />
    </svg>
  );
}
