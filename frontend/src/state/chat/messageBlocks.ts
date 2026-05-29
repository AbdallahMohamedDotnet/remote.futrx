// Convert a flat stream of ChatEvents into renderable message blocks.
// Each "user" event starts a User block; everything in between (assistant_text,
// tool_use_start/end, thinking) belongs to the trailing Assistant block.

import type { ChatEvent } from "../../models/chat";

export type AssistantPart =
  | { kind: "text"; text: string }
  | {
      kind: "tool";
      id: string;
      name: string;
      input: Record<string, unknown>;
      output?: string;
      isError?: boolean;
      status: "running" | "done";
    }
  | { kind: "thinking"; text: string };

export type AssistantBlock = { type: "assistant"; parts: AssistantPart[]; t: number; isComplete: boolean };
export type Block =
  | { type: "user"; text: string; t: number }
  | AssistantBlock
  | { type: "error"; message: string; t: number };

export function groupEvents(events: ChatEvent[]): Block[] {
  const blocks: Block[] = [];
  // Track the trailing assistant block by index so TS can narrow cleanly.
  let curIdx = -1;

  const getCur = (): AssistantBlock | null => {
    if (curIdx < 0) return null;
    const b = blocks[curIdx];
    return b.type === "assistant" ? b : null;
  };
  const ensureAssistant = (t: number): AssistantBlock => {
    const c = getCur();
    if (c) return c;
    const fresh: AssistantBlock = { type: "assistant", parts: [], t, isComplete: false };
    blocks.push(fresh);
    curIdx = blocks.length - 1;
    return fresh;
  };
  const endAssistant = () => {
    const c = getCur();
    if (c) c.isComplete = true;
    curIdx = -1;
  };

  for (const ev of events) {
    switch (ev.type) {
      case "user":
        endAssistant();
        blocks.push({ type: "user", text: ev.text, t: ev.t });
        break;
      case "assistant_text": {
        const a = ensureAssistant(ev.t);
        const last = a.parts[a.parts.length - 1];
        if (last && last.kind === "text") {
          last.text += ev.text;
        } else {
          a.parts.push({ kind: "text", text: ev.text });
        }
        break;
      }
      case "thinking": {
        const a = ensureAssistant(ev.t);
        a.parts.push({ kind: "thinking", text: ev.text });
        break;
      }
      case "tool_use_start": {
        const a = ensureAssistant(ev.t);
        a.parts.push({
          kind: "tool",
          id: ev.id,
          name: ev.name,
          input: (ev.input as unknown as Record<string, unknown>) ?? {},
          status: "running",
        });
        break;
      }
      case "tool_use_end": {
        const c = getCur();
        if (c) {
          const tool = c.parts.find(
            (p): p is Extract<AssistantPart, { kind: "tool" }> =>
              p.kind === "tool" && p.id === ev.id
          );
          if (tool) {
            tool.output = ev.output;
            tool.isError = ev.isError;
            tool.status = "done";
          }
        }
        break;
      }
      case "complete":
        endAssistant();
        break;
      case "error":
        endAssistant();
        blocks.push({ type: "error", message: ev.message, t: ev.t });
        break;
      // session, system — intentionally not rendered
    }
  }

  return blocks;
}
