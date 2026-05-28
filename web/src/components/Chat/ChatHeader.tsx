import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import type { ChatEvent, ChatMeta } from "../../types";
import { chatsApi } from "../../lib/api";
import { shortenPath } from "../../lib/format";
import { ChevronDown, Folder, Menu, MessageSquare } from "../icons";

interface Props {
  chat: ChatMeta;
  events: ChatEvent[];
  streaming: boolean;
  onModelChange: (model: string) => void;
  onCwdChange: (cwd: string) => void;
  onHamburger: () => void;
}

const MODEL_OPTIONS: Array<{ value: string; label: string; sub: string }> = [
  { value: "opus",   label: "Opus",   sub: "deepest reasoning" },
  { value: "sonnet", label: "Sonnet", sub: "balanced" },
  { value: "haiku",  label: "Haiku",  sub: "fast & cheap" },
];

function modelDisplayLabel(m?: string): string {
  if (!m) return "Auto";
  const lower = m.toLowerCase();
  if (lower.includes("opus"))   return "Opus";
  if (lower.includes("sonnet")) return "Sonnet";
  if (lower.includes("haiku"))  return "Haiku";
  return m;
}

// Pluck the latest complete event's usage stats. claude returns:
//   input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens
type Usage = {
  input_tokens?: number;
  output_tokens?: number;
  cache_read_input_tokens?: number;
  cache_creation_input_tokens?: number;
} | null;

function fmtTokens(n?: number): string {
  if (!n && n !== 0) return "—";
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(n >= 10_000 ? 0 : 1) + "k";
  return String(n);
}

// Rough cost estimate (USD per 1M tokens). Used for live readouts only; the
// canonical figure is whatever claude reports via total_cost_usd in result.
const COST = {
  opus:   { in: 15.0,  out: 75.0, cacheRead: 1.50, cacheWrite: 18.75 },
  sonnet: { in:  3.0,  out: 15.0, cacheRead: 0.30, cacheWrite:  3.75 },
  haiku:  { in:  0.8,  out:  4.0, cacheRead: 0.08, cacheWrite:  1.00 },
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

  // Close model dropdown on outside click
  useEffect(() => {
    if (!modelOpen) return;
    const h = (e: MouseEvent) => {
      if (modelRef.current && !modelRef.current.contains(e.target as Node)) setModelOpen(false);
    };
    window.addEventListener("mousedown", h);
    return () => window.removeEventListener("mousedown", h);
  }, [modelOpen]);

  // Aggregate usage across all "complete" events in this chat.
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
    { input_tokens: totals.inT, output_tokens: totals.outT,
      cache_read_input_tokens: totals.cacheR, cache_creation_input_tokens: totals.cacheW },
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

  return (
    <header class="bg-ink-700 border-b border-ink-500 px-3 py-2 flex flex-col gap-1.5">
      <div class="flex items-center gap-2 min-h-[28px]">
        <button
          type="button"
          onClick={onHamburger}
          class="md:hidden text-ink-100 p-1 rounded hover:bg-ink-600 flex-none"
          aria-label="Toggle sidebar"
        >
          <Menu class="w-5 h-5" />
        </button>
        <MessageSquare class="w-4 h-4 text-ink-300 flex-none" />
        <span class="text-sm text-ink-100 truncate flex-1 font-medium">
          {chat.title || "Untitled chat"}
        </span>

        {/* Model dropdown */}
        <div ref={modelRef} class="relative flex-none">
          <button
            type="button"
            onClick={() => setModelOpen((o) => !o)}
            class="flex items-center gap-1 text-xs font-mono px-2 py-1 rounded
                   bg-ink-600 hover:bg-ink-500 border border-ink-500 text-ink-100"
            disabled={streaming}
            title={streaming ? "Cannot change model mid-stream" : "Switch model"}
          >
            <span>{modelDisplayLabel(chat.model)}</span>
            <ChevronDown class="w-3 h-3 text-ink-300" />
          </button>
          {modelOpen && (
            <div class="absolute right-0 top-full mt-1 z-30 min-w-[200px]
                        bg-ink-700 border border-ink-500 rounded-md shadow-xl
                        py-1 text-sm">
              {MODEL_OPTIONS.map((m) => {
                const active = modelDisplayLabel(chat.model).toLowerCase() === m.value;
                return (
                  <button
                    key={m.value}
                    type="button"
                    onClick={() => pickModel(m.value)}
                    class={`w-full flex flex-col items-start gap-0 px-3 py-1.5 text-left
                            ${active ? "bg-accent-blue/15 text-accent-blue" : "hover:bg-ink-600 text-ink-100"}`}
                  >
                    <span class="text-[13px]">{m.label}</span>
                    <span class="text-[11px] text-ink-300">{m.sub}</span>
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {/* Sub-row: cwd + token + cost */}
      <div class="flex items-center gap-3 text-[11px] text-ink-300 font-mono">
        <Folder class="w-3 h-3 text-accent-blue flex-none" />
        {editingCwd ? (
          <input
            class="flex-1 min-w-0 bg-ink-800 border border-ink-500 rounded px-1.5 py-0.5
                   text-ink-100 focus:outline-none focus:border-accent-blue"
            value={cwdInput}
            onInput={(e) => setCwdInput((e.currentTarget as HTMLInputElement).value)}
            onBlur={commitCwd}
            onKeyDown={(e) => { if (e.key === "Enter") commitCwd(); if (e.key === "Escape") { setCwdInput(chat.cwd ?? ""); setEditingCwd(false); } }}
            autofocus
          />
        ) : (
          <button
            type="button"
            class="flex-1 min-w-0 text-left truncate text-ink-200 hover:text-ink-100"
            onClick={() => setEditingCwd(true)}
            title="Click to change working directory"
            style={{ direction: "rtl", textAlign: "left", unicodeBidi: "plaintext" }}
          >
            {shortenPath(chat.cwd || "~")}
          </button>
        )}

        <div class="flex items-center gap-2 flex-none">
          <span title={`Input ${totals.inT}\nOutput ${totals.outT}\nCache read ${totals.cacheR}\nCache write ${totals.cacheW}`}>
            <span class="text-ink-200">↑</span> {fmtTokens(totals.inT)}
            {" "}
            <span class="text-ink-200">↓</span> {fmtTokens(totals.outT)}
            {totals.cacheR > 0 && (
              <> {" "} <span class="text-ink-200">⚡</span> {fmtTokens(totals.cacheR)}</>
            )}
          </span>
          {costUsd > 0 && (
            <span class="text-ink-200">${costUsd.toFixed(costUsd < 0.01 ? 4 : 2)}</span>
          )}
        </div>
      </div>
    </header>
  );
}
