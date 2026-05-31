import type { ComponentChildren } from "preact";
import type { AssistantPart, Block } from "../../state/chat/messageBlocks";
import { AlertCircle, Loader, RotateCcw, TerminalIcon } from "../ui/icons";
import { StreamingText } from "./StreamingText";
import { ToolCall } from "./tool-calls/ToolCall";
import { ToolShell } from "./tool-calls/ToolShell";

type ToolPart = Extract<AssistantPart, { kind: "tool" }>;

interface RenderContext {
  streaming: boolean;
  chatId?: string;
  onAnswerQuestion?: (text: string) => void;
}

export function MessageBlock({
  block,
  streaming,
  chatId,
  onAnswerQuestion,
  onRewind,
}: {
  block: Block;
  streaming: boolean;
  chatId?: string;
  onAnswerQuestion?: (text: string) => void;
  onRewind?: (t: number, text: string) => void;
}) {
  if (block.type === "user") {
    return (
      <div class="group flex justify-end">
        <div class="max-w-[92%] sm:max-w-[78%] flex flex-col items-end gap-1.5">
          <div class="codex-user-bubble bg-accent-blue/15 border border-accent-blue/30
                      rounded-[18px] rounded-br-md px-3.5 py-2.5 text-[14.5px] leading-relaxed
                      whitespace-pre-wrap break-words shadow-sm">
            {block.text}
          </div>
          {onRewind && (
            <button
              type="button"
              onClick={() => onRewind(block.t, block.text)}
              class="inline-flex items-center gap-1.5 h-7 px-2 rounded-md text-[12px]
                     text-ink-300 hover:text-ink-100 hover:bg-white/[0.07]
                     opacity-100 md:opacity-0 md:group-hover:opacity-100 transition"
              title="Rewind and edit from here"
            >
              <RotateCcw class="w-3.5 h-3.5" />
              Rewind
            </button>
          )}
        </div>
      </div>
    );
  }

  if (block.type === "error") {
    return (
      <div class="flex items-start gap-2 text-accent-red text-sm rounded-lg border border-accent-red/25 bg-accent-red/[0.08] px-3 py-2">
        <AlertCircle class="w-4 h-4 flex-none mt-0.5" />
        <div>{block.message}</div>
      </div>
    );
  }

  return (
    <div class="codex-assistant-block space-y-2 max-w-full">
      {renderAssistantParts(block.parts, { streaming, chatId, onAnswerQuestion })}
      {streaming && !block.isComplete && (
        <div class="inline-flex items-center gap-2 text-ink-300 text-xs pt-1 rounded-full bg-white/5 px-2.5 py-1">
          <Loader class="w-3 h-3 animate-spin" />
          thinking
        </div>
      )}
    </div>
  );
}

function renderAssistantParts(parts: AssistantPart[], context: RenderContext): ComponentChildren[] {
  const rendered: ComponentChildren[] = [];
  let toolGroup: ToolPart[] = [];
  let toolGroupStart = 0;

  const flushToolGroup = () => {
    if (!toolGroup.length) return;
    rendered.push(
      <ToolGroup
        key={`tools-${toolGroupStart}`}
        parts={toolGroup}
        startIndex={toolGroupStart}
        chatId={context.chatId}
        onAnswerQuestion={context.onAnswerQuestion}
      />
    );
    toolGroup = [];
  };

  parts.forEach((part, index) => {
    if (part.kind === "tool" && isGroupableTool(part)) {
      if (!toolGroup.length) toolGroupStart = index;
      toolGroup.push(part);
      return;
    }

    flushToolGroup();

    if (part.kind === "text") {
      rendered.push(
        <div key={index} class="text-[15px] leading-7 text-ink-100">
          <StreamingText text={part.text} streaming={context.streaming} />
        </div>
      );
      return;
    }

    if (part.kind === "thinking") {
      rendered.push(
        <div key={index} class="text-[13px] italic text-ink-300 border-l-2 border-accent-yellow/[0.45] pl-3 my-2">
          {part.text}
        </div>
      );
      return;
    }

    if (part.kind === "tool") {
      rendered.push(renderToolCall(part, part.id || index, context));
    }
  });

  flushToolGroup();
  return rendered;
}

function isGroupableTool(part: ToolPart): boolean {
  return part.kind === "tool" && part.name !== "AskUserQuestion";
}

function ToolGroup({
  parts,
  startIndex,
  chatId,
  onAnswerQuestion,
}: {
  parts: ToolPart[];
  startIndex: number;
  chatId?: string;
  onAnswerQuestion?: (text: string) => void;
}) {
  const status = parts.some((part) => part.status === "running") ? "running" : "done";
  const isError = parts.some((part) => part.isError);
  const count = parts.length;
  const label = `${count} ${count === 1 ? "tool" : "tools"} used`;

  return (
    <ToolShell
      icon={<TerminalIcon class="w-4 h-4" />}
      label={<span class="font-medium">{label}</span>}
      status={status}
      isError={isError}
    >
      <div class="codex-tool-group-list p-2">
        {parts.map((part, offset) =>
          renderToolCall(part, part.id || `${startIndex}-${offset}`, { chatId, onAnswerQuestion, streaming: false })
        )}
      </div>
    </ToolShell>
  );
}

function renderToolCall(part: ToolPart, key: string | number, context: RenderContext) {
  return (
    <ToolCall
      key={key}
      toolUseId={part.id}
      chatId={context.chatId}
      name={part.name}
      input={part.input}
      output={part.output}
      isError={part.isError}
      status={part.status}
      onAnswerQuestion={context.onAnswerQuestion}
    />
  );
}
