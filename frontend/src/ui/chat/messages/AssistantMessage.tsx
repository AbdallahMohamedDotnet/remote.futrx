import type { AssistantBlock } from "../../../models/chatMessage";
import { AssistantPartList } from "./AssistantPartList";
import { ThinkingIndicator } from "./ThinkingIndicator";

export function AssistantMessage({
  block,
  streaming,
  chatId,
  cwd,
  onAnswerQuestion,
}: {
  block: AssistantBlock;
  streaming: boolean;
  chatId?: string;
  cwd?: string;
  onAnswerQuestion?: (text: string) => void;
}) {
  return (
    <div class="codex-assistant-block space-y-2 max-w-full">
      <AssistantPartList
        parts={block.parts}
        streaming={streaming}
        chatId={chatId}
        cwd={cwd}
        onAnswerQuestion={onAnswerQuestion}
      />
      {streaming && !block.isComplete && <ThinkingIndicator />}
    </div>
  );
}
