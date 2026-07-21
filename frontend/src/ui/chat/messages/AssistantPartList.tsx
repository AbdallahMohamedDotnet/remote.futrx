import type { ComponentChildren } from "preact";
import type { AssistantPart } from "../../../state/chat/messageBlocks";
import { ToolCall } from "../tool-calls/ToolCall";
import { StreamingText } from "./StreamingText";
import { ToolGroup } from "./ToolGroup";

type ToolPart = Extract<AssistantPart, { kind: "tool" }>;

export function AssistantPartList({
  parts,
  streaming,
  chatId,
  cwd,
  onAnswerQuestion,
}: {
  parts: AssistantPart[];
  streaming: boolean;
  chatId?: string;
  cwd?: string;
  onAnswerQuestion?: (text: string) => void;
}) {
  return <>{renderAssistantParts(parts, { streaming, chatId, cwd, onAnswerQuestion })}</>;
}

function renderAssistantParts(
  parts: AssistantPart[],
  context: { streaming: boolean; chatId?: string; cwd?: string; onAnswerQuestion?: (text: string) => void }
): ComponentChildren[] {
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
          <StreamingText text={part.text} streaming={context.streaming} chatId={context.chatId} cwd={context.cwd} />
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

    rendered.push(
      <ToolCall
        key={part.id || index}
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
  });

  flushToolGroup();
  return rendered;
}

function isGroupableTool(part: ToolPart): boolean {
  return part.name !== "AskUserQuestion";
}
