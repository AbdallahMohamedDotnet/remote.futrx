import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import type { ChatEvent, ChatMeta } from "../../types";
import { shortenPath } from "../../lib/format";
import { Activity, ChevronDown, Folder, Menu, MessageSquare } from "../icons";

interface Props {
  chat: ChatMeta;
  events: ChatEvent[];
  streaming: boolean;
  onModelChange: (model: string) => void;
  onCwdChange: (cwd: string) => void;
  onHamburger: () => void;
}

const MODEL_OPTIONS: Array<{ value: string; label: string; sub: string }> = [
  { value: "", label: "Auto", sub: "server default" },
  { value: "opus", label: "Opus", sub: "deepest reasoning" },
  { value: "sonnet", label: "Sonnet", sub: "balanced" },
  { value: "haiku", label: "Haiku", sub: "fast" },
];

function modelDisplayLabel(m?: string): string {
  if (!m) return "Auto";
  const lower = m.toLowerCase();
  if (lower.includes("opus")) return "Opus";
  if (lower.includes("sonnet")) return "Sonnet";
  if (lower.includes("haiku")) return "Haiku";
  return m;
}

type Usage = {
  input_tokens?: number;
  output_tokens?: number;
  cache_read_input_tokens?: number;
  cache_creation_input_tokens?: number;
} | null;

function fmtTokens(n?: number): string {
  if (!n && n !== 0) return "0";
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(n >= 10_000 ? 0 : 1) + "k";
  return String(n);
}

const COST = {
  opus: { in: 15.0, out: 75.0, cacheRead: 1.5, cacheWrite: 18.75 },
  sonnet: { in: 3.0, out: 15.0, cacheRead: 0.3, cacheWrite: 3.75 },
  haiku: { in: 0.8, out: 4.0, cacheRead: 0.08, cacheWrite: 1.0 },
};

function estimateCost(u: Usage, model: string): number {
  if (!u) return 0;
  const key = (modelDisplayLabel(model).toLowerCase() as keyof typeof COST);
  const c = COST[key] ?? COST.sonnet;
  return (
    ((u.input_tokens ?? 0) * c.in +
      (u.output_tokens ?? 0) * c.out +
      (u.cache_read_input_tokens ?? 0) * c.cacheRead +
      (u.cache_creation_input_tokens ?? 0) * c.cacheWrite) / 1_000_000
  );
}

export function ChatHeader({ chat, events, streaming, onModelChange, onCwdChange, onHamburger }: Props) {
  const [modelOpen, setModelOpen] = useState(false);
  const [editingCwd, setEditingCwd] = useState(false);
  const [cwdInput, setCwdInput] = useState(chat.cwd ?? "");
  const modelRef = useRef<HTMLDivElement>(null);

  useEffect(() => { setCwdInput(chat.cwd ?? ""); }, [chat.cwd]);

  useEffect(() => {
    if (!modelOpen) return;
    const h = (e: MouseEvent) => {
      if (modelRef.current && !modelRef.current.contains(e.target as Node)) setModelOpen(false);
    };
    window.addEventListener("mousedown", h);
    return () => window.removeEventListener("mousedown", h);
  }, [modelOpen]);

  const totals = useMemo(() => {
    let inT = 0, outT = 0, cacheR = 0, cacheW = 0;
    for (const ev of events) {
      if (ev.type !== "complete" || !ev.usage) continue;
      try {
        const u = (typeof ev.usage === "string" ? JSON.parse(ev.usage) : ev.usage) as Usage;
        if (!u) continue;
        inT += u.input_tokens ?? 0;
        outT += u.output_tokens ?? 0;
        cacheR += u.cache_read_input_tokens ?? 0;
        cacheW += u.cache_creation_input_tokens ?? 0;
      } catch {}
    }
    return { inT, outT, cacheR, cacheW };
  }, [events]);

  const costUsd = estimateCost(
    {
      input_tokens: totals.inT,
      output_tokens: totals.outT,
      cache_read_input_tokens: totals.cacheR,
      cache_creation_input_tokens: totals.cacheW,
    },
    chat.model || ""
  );

  function pickModel(v: string) {
    setModelOpen(false);
    if (v !== chat.model) onModelChange(v);
  }

  async function commitCwd() {
    const v = cwdInput.trim();
    setEditingCwd(false);
    if (v !== (chat.cwd ?? "")) onCwdChange(v);
  }

  const tokenTotal = totals.inT + totals.outT + totals.cacheR + totals.cacheW;

  return (
    <header class="codex-header top-chrome flex-none sticky top-0 z-20 bg-[#101318] md:bg-[#101318]/95 md:backdrop-blur border-b border-white/10 px-3 md:px-4 pb-2 flex flex-col gap-2">
      <div class="flex items-center gap-2 min-h-11">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden h-10 w-10 rounded-md text-ink-100 hover:bg-white/[0.08] grid place-items-center flex-none"
          aria-label="Open chats"
          title="Chats"
        >
          <Menu class="w-5 h-5" />
        </button>

        <div class="hidden sm:grid h-9 w-9 rounded-md bg-white/[0.06] border border-white/10 text-ink-200 place-items-center flex-none">
          <MessageSquare class="w-4 h-4" />
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 min-w-0">
            <h1 class="truncate text-[15px] md:text-base font-semibold text-ink-50">
              {chat.title || "Untitled chat"}
            </h1>
            <span
              class={`h-2 w-2 rounded-full flex-none ${streaming ? "bg-accent-green animate-pulse" : "bg-ink-400"}`}
              title={streaming ? "Streaming" : "Ready"}
            />
          </div>
          <div class="text-[12px] text-ink-300 truncate">
            {streaming ? "Working" : "Ready"}
          </div>
        </div>

        <div ref={modelRef} class="relative flex-none">
          <button
            type="button"
            onClick={() => setModelOpen((o) => !o)}
            class="h-9 inline-flex items-center justify-center gap-1.5 text-[13px] font-medium px-2.5 sm:px-3 rounded-md
                   bg-white/[0.06] hover:bg-white/10 border border-white/10 text-ink-100 disabled:opacity-50"
            disabled={streaming}
            title={streaming ? "Cannot change model while streaming" : "Switch model"}
          >
            <span>{modelDisplayLabel(chat.model)}</span>
            <ChevronDown class="w-3.5 h-3.5 text-ink-300" />
          </button>
          {modelOpen && (
            <div class="absolute right-0 top-full mt-2 z-40 w-[220px]
                        bg-[#151922] border border-white/[0.12] rounded-lg shadow-2xl overflow-hidden p-1">
              {MODEL_OPTIONS.map((m) => {
                const active = (chat.model || "") === m.value ||
                  (m.value !== "" && modelDisplayLabel(chat.model).toLowerCase() === m.value);
                return (
                  <button
                    key={m.value}
                    type="button"
                    onClick={() => pickModel(m.value)}
                    class={`w-full flex items-center justify-between gap-3 px-3 py-2.5 rounded-md text-left
                            ${active ? "bg-accent-blue/[0.16] text-accent-blue" : "hover:bg-white/[0.07] text-ink-100"}`}
                  >
                    <span>
                      <span class="block text-[14px] font-medium">{m.label}</span>
                      <span class="block text-[12px] text-ink-300">{m.sub}</span>
                    </span>
                    {active && <span class="h-2 w-2 rounded-full bg-accent-blue" />}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </div>

      <div class="flex items-center gap-2 overflow-x-auto no-scrollbar pb-0.5">
        {editingCwd ? (
          <input
            class="h-9 flex-1 min-w-[220px] bg-[#0b0d11] border border-accent-blue/70 rounded-md px-3
                   text-ink-100 text-[13px] font-mono focus:outline-none"
            value={cwdInput}
            onInput={(e) => setCwdInput((e.currentTarget as HTMLInputElement).value)}
            onBlur={commitCwd}
            onKeyDown={(e) => {
              if (e.key === "Enter") commitCwd();
              if (e.key === "Escape") {
                setCwdInput(chat.cwd ?? "");
                setEditingCwd(false);
              }
            }}
            autofocus
          />
        ) : (
          <button
            type="button"
            class="h-9 max-w-[72vw] md:max-w-[520px] inline-flex items-center gap-2 px-3 rounded-md
                   bg-white/5 hover:bg-white/[0.09] border border-white/10 text-left text-ink-200"
            onClick={() => setEditingCwd(true)}
            title="Change working directory"
          >
            <Folder class="w-4 h-4 text-accent-blue flex-none" />
            <span class="truncate font-mono text-[12.5px]">{shortenPath(chat.cwd || "~")}</span>
          </button>
        )}

        <div
          class="h-9 inline-flex items-center gap-2 px-3 rounded-md bg-white/5 border border-white/10
                 text-[12.5px] text-ink-300 flex-none"
          title={`Input ${totals.inT}\nOutput ${totals.outT}\nCache read ${totals.cacheR}\nCache write ${totals.cacheW}`}
        >
          <Activity class="w-4 h-4 text-accent-green" />
          <span>{fmtTokens(tokenTotal)} tokens</span>
          {costUsd > 0 && <span class="text-ink-100">${costUsd.toFixed(costUsd < 0.01 ? 4 : 2)}</span>}
        </div>
      </div>
    </header>
  );
}
