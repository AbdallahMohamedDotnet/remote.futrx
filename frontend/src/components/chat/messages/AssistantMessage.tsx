import type { AssistantBlock } from "../../../state/chat/messageBlocks";
import { AssistantPartList } from "./AssistantPartList";
import { ThinkingIndicator } from "./ThinkingIndicator";

export function AssistantMessage({
  block,
  streaming,
  chatId,
  onAnswerQuestion,
}: {
  block: AssistantBlock;
  streaming: boolean;
  chatId?: string;
  onAnswerQuestion?: (text: string) => void;
}) {
  return (
    <div class="codex-assistant-block space-y-2 max-w-full">
      <AssistantPartList
        parts={block.parts}
        streaming={streaming}
        chatId={chatId}
        onAnswerQuestion={onAnswerQuestion}
      />
      {streaming && !block.isComplete && <ThinkingIndicator />}
    </div>
  );
}
