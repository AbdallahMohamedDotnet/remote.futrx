import type {
  AnswerQuestionHandler,
  InteractionActivityHandler,
} from "../../../models/chat";
import type { AssistantMessageBlock } from "../../../models/chatMessage";
import { AssistantPartList } from "./AssistantPartList";
import { ThinkingIndicator } from "./ThinkingIndicator";

export function AssistantMessage({
  block,
  streaming,
  chatId,
  cwd,
  onAnswerQuestion,
  onInteractionActivity,
}: {
  block: AssistantMessageBlock;
  streaming: boolean;
  chatId?: string;
  cwd?: string;
  onAnswerQuestion?: AnswerQuestionHandler;
  onInteractionActivity?: InteractionActivityHandler;
}) {
  return (
    <div class="codex-assistant-block space-y-2 max-w-full">
      <AssistantPartList
        parts={block.parts}
        streaming={streaming}
        chatId={chatId}
        cwd={cwd}
        onAnswerQuestion={onAnswerQuestion}
        onInteractionActivity={onInteractionActivity}
      />
      {streaming && !block.isComplete && <ThinkingIndicator />}
    </div>
  );
}
