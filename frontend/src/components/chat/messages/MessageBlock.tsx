import type { Block } from "../../../state/chat/messageBlocks";
import { AssistantMessage } from "./AssistantMessage";
import { ErrorMessage } from "./ErrorMessage";
import { UserMessage } from "./UserMessage";

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
    return <UserMessage text={block.text} t={block.t} onRewind={onRewind} />;
  }

  if (block.type === "error") {
    return <ErrorMessage message={block.message} />;
  }

  return (
    <AssistantMessage
      block={block}
      streaming={streaming}
      chatId={chatId}
      onAnswerQuestion={onAnswerQuestion}
    />
  );
}
