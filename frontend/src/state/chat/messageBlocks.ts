// Convert a flat stream of ChatEvents into renderable message blocks.
// Each "user" event starts a User block; everything in between (assistant_text,
// tool_use_start/end, thinking) belongs to the trailing Assistant block.

import type { ChatEvent } from "../../models/chat";
import type {
  AssistantMessageBlock,
  AssistantMessagePart,
  ChatMessageBlock,
} from "../../models/chatMessage";

export function buildChatMessageBlocks(events: ChatEvent[]): ChatMessageBlock[] {
  return events.reduce(appendEventToBlocks, [] as ChatMessageBlock[]);
}

function appendEventToBlocks(blocks: ChatMessageBlock[], event: ChatEvent): ChatMessageBlock[] {
  switch (event.type) {
    case "user": {
      const next = endTrailingAssistant(blocks);
      return [...next, { type: "user", text: event.text, t: event.t }];
    }
    case "assistant_text": {
      const { blocks: next, assistant } = ensureTrailingAssistant(blocks, event.t);
      const lastIndex = assistant.parts.length - 1;
      const last = assistant.parts[lastIndex];
      if (last?.kind === "text") {
        assistant.parts[lastIndex] = { ...last, text: last.text + event.text };
      } else {
        assistant.parts.push({ kind: "text", text: event.text });
      }
      return next;
    }
    case "thinking": {
      const { blocks: next, assistant } = ensureTrailingAssistant(blocks, event.t);
      assistant.parts.push({ kind: "thinking", text: event.text });
      return next;
    }
    case "tool_use_start": {
      const { blocks: next, assistant } = ensureTrailingAssistant(blocks, event.t);
      assistant.parts.push({
        kind: "tool",
        id: event.id,
        name: event.name,
        input: (event.input as unknown as Record<string, unknown>) ?? {},
        status: "running",
      });
      return next;
    }
    case "tool_use_end":
      return updateTrailingTool(blocks, event.id, {
        output: event.output,
        isError: event.isError,
        status: "done",
      });
    case "complete":
      return endTrailingAssistant(blocks);
    case "error": {
      const next = endTrailingAssistant(blocks);
      return [...next, { type: "error", message: event.message, t: event.t }];
    }
    // sync, session, system, permission_request — intentionally not rendered.
    default:
      return blocks;
  }
}

function endTrailingAssistant(blocks: ChatMessageBlock[]): ChatMessageBlock[] {
  const lastIndex = blocks.length - 1;
  const last = blocks[lastIndex];
  if (!last || last.type !== "assistant" || last.isComplete) return blocks;
  const next = blocks.slice();
  next[lastIndex] = { ...last, isComplete: true };
  return next;
}

function ensureTrailingAssistant(
  blocks: ChatMessageBlock[],
  t: number
): { blocks: ChatMessageBlock[]; assistant: AssistantMessageBlock } {
  const lastIndex = blocks.length - 1;
  const last = blocks[lastIndex];
  if (last?.type === "assistant" && !last.isComplete) {
    const next = blocks.slice();
    const assistant: AssistantMessageBlock = { ...last, parts: last.parts.slice() };
    next[lastIndex] = assistant;
    return { blocks: next, assistant };
  }

  const assistant: AssistantMessageBlock = { type: "assistant", parts: [], t, isComplete: false };
  return { blocks: [...blocks, assistant], assistant };
}

function updateTrailingTool(
  blocks: ChatMessageBlock[],
  id: string,
  patch: Partial<Extract<AssistantMessagePart, { kind: "tool" }>>
): ChatMessageBlock[] {
  const lastIndex = blocks.length - 1;
  const last = blocks[lastIndex];
  if (!last || last.type !== "assistant") return blocks;

  const partIndex = last.parts.findIndex((part) => part.kind === "tool" && part.id === id);
  if (partIndex < 0) return blocks;

  const next = blocks.slice();
  const parts = last.parts.slice();
  parts[partIndex] = { ...parts[partIndex], ...patch } as AssistantMessagePart;
  next[lastIndex] = { ...last, parts };
  return next;
}
