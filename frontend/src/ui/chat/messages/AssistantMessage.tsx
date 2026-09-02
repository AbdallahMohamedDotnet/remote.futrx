import type { AssistantMessageBlock } from "../../../models/chatMessage";
import { AssistantPartList } from "./AssistantPartList";
import { ThinkingIndicator } from "./ThinkingIndicator";

export function AssistantMessage({
  block,
  streaming,
  chatId,
  cwd,
  onAnswerQuestion,
  onRespondInteraction,
}: {
  block: AssistantMessageBlock;
  streaming: boolean;
  chatId?: string;
  cwd?: string;
  onAnswerQuestion?: (text: string) => void;
  onRespondInteraction?: (interactionId: string, result?: unknown, error?: unknown) => boolean;
}) {
  return (
    <div class="codex-assistant-block space-y-2 max-w-full">
      <AssistantPartList
        parts={block.parts}
        streaming={streaming}
        chatId={chatId}
        cwd={cwd}
        onAnswerQuestion={onAnswerQuestion}
        onRespondInteraction={onRespondInteraction}
      />
      {streaming && !block.isComplete && <ThinkingIndicator />}
    </div>
  );
}
