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
  return events.reduce(appendEventToBlocks, [] as Block[]);
}

export function appendEventToBlocks(blocks: Block[], ev: ChatEvent): Block[] {
  switch (ev.type) {
    case "user": {
      const next = endTrailingAssistant(blocks);
      return [...next, { type: "user", text: ev.text, t: ev.t }];
    }
    case "assistant_text": {
      const { blocks: next, assistant } = ensureTrailingAssistant(blocks, ev.t);
      const lastIndex = assistant.parts.length - 1;
      const last = assistant.parts[lastIndex];
      if (last?.kind === "text") {
        assistant.parts[lastIndex] = { ...last, text: last.text + ev.text };
      } else {
        assistant.parts.push({ kind: "text", text: ev.text });
      }
      return next;
    }
    case "thinking": {
      const { blocks: next, assistant } = ensureTrailingAssistant(blocks, ev.t);
      assistant.parts.push({ kind: "thinking", text: ev.text });
      return next;
    }
    case "tool_use_start": {
      const { blocks: next, assistant } = ensureTrailingAssistant(blocks, ev.t);
      assistant.parts.push({
        kind: "tool",
        id: ev.id,
        name: ev.name,
        input: (ev.input as unknown as Record<string, unknown>) ?? {},
        status: "running",
      });
      return next;
    }
    case "tool_use_end":
      return updateTrailingTool(blocks, ev.id, {
        output: ev.output,
        isError: ev.isError,
        status: "done",
      });
    case "complete":
      return endTrailingAssistant(blocks);
    case "error": {
      const next = endTrailingAssistant(blocks);
      return [...next, { type: "error", message: ev.message, t: ev.t }];
    }
    // sync, session, system, permission_request — intentionally not rendered.
    default:
      return blocks;
  }
}

function endTrailingAssistant(blocks: Block[]): Block[] {
  const lastIndex = blocks.length - 1;
  const last = blocks[lastIndex];
  if (!last || last.type !== "assistant" || last.isComplete) return blocks;
  const next = blocks.slice();
  next[lastIndex] = { ...last, isComplete: true };
  return next;
}

function ensureTrailingAssistant(
  blocks: Block[],
  t: number
): { blocks: Block[]; assistant: AssistantBlock } {
  const lastIndex = blocks.length - 1;
  const last = blocks[lastIndex];
  if (last?.type === "assistant" && !last.isComplete) {
    const next = blocks.slice();
    const assistant: AssistantBlock = { ...last, parts: last.parts.slice() };
    next[lastIndex] = assistant;
    return { blocks: next, assistant };
  }

  const assistant: AssistantBlock = { type: "assistant", parts: [], t, isComplete: false };
  return { blocks: [...blocks, assistant], assistant };
}

function updateTrailingTool(
  blocks: Block[],
  id: string,
  patch: Partial<Extract<AssistantPart, { kind: "tool" }>>
): Block[] {
  const lastIndex = blocks.length - 1;
  const last = blocks[lastIndex];
  if (!last || last.type !== "assistant") return blocks;

  const partIndex = last.parts.findIndex((part) => part.kind === "tool" && part.id === id);
  if (partIndex < 0) return blocks;

  const next = blocks.slice();
  const parts = last.parts.slice();
  parts[partIndex] = { ...parts[partIndex], ...patch } as AssistantPart;
  next[lastIndex] = { ...last, parts };
  return next;
}
