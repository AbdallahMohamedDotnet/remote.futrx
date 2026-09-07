import { AskUserQuestion } from "./ask-user-question/AskUserQuestion";
import type { AskInput, ToolCallProps } from "./ToolCallTypes";
import { useState } from "preact/hooks";
import { chatApi } from "../../../api/chatApi";
import { BashCall } from "./renderers/BashCall";
import { EditCall } from "./renderers/EditCall";
import { GenericCall } from "./renderers/GenericCall";
import { ReadCall } from "./renderers/ReadCall";
import { SearchCall } from "./renderers/SearchCall";
import { WriteCall } from "./renderers/WriteCall";

export function ToolCall(props: ToolCallProps) {
  const {
    toolUseId,
    chatId,
    name,
    input,
    output,
    outputRef,
    outputBytes,
    onAnswerQuestion,
  } = props;
  const [fullOutput, setFullOutput] = useState<string | null>(null);
  const [loadingOutput, setLoadingOutput] = useState(false);
  const [outputError, setOutputError] = useState<string | null>(null);
  const canExpandInline = !outputRef && !!output && output.length > 6000;

  if (name === "AskUserQuestion" && toolUseId && chatId && onAnswerQuestion) {
    return (
      <AskUserQuestion
        toolUseId={toolUseId}
        chatId={chatId}
        input={(input as unknown as AskInput) ?? { questions: [] }}
        onSubmit={onAnswerQuestion}
      />
    );
  }

  const rendererProps = {
    ...props,
    output: fullOutput ?? output,
    outputExpanded: fullOutput !== null,
  };
  let rendered;
  switch (name) {
    case "Read":
      rendered = <ReadCall {...rendererProps} />;
      break;
    case "Edit":
    case "MultiEdit":
      rendered = <EditCall {...rendererProps} />;
      break;
    case "Write":
      rendered = <WriteCall {...rendererProps} />;
      break;
    case "Bash":
      rendered = <BashCall {...rendererProps} />;
      break;
    case "Glob":
    case "Grep":
      rendered = <SearchCall {...rendererProps} />;
      break;
    default:
      rendered = <GenericCall {...rendererProps} />;
  }

  async function loadFullOutput() {
    if (loadingOutput) return;
    if (!outputRef) {
      if (output) setFullOutput(output);
      return;
    }
    if (!chatId) return;
    setLoadingOutput(true);
    setOutputError(null);
    try {
      setFullOutput(await chatApi.fetchFullTranscriptContent(chatId, outputRef));
    } catch (error) {
      setOutputError((error as Error).message);
    } finally {
      setLoadingOutput(false);
    }
  }

  return (
    <>
      {rendered}
      {(outputRef || canExpandInline) && fullOutput === null && (
        <div class="-mt-1 mb-2 flex items-center gap-2 px-2 text-[11px]">
          <button
            type="button"
            disabled={(!!outputRef && !chatId) || loadingOutput}
            onClick={() => void loadFullOutput()}
            class="text-accent-blue hover:underline disabled:opacity-50"
          >
            {loadingOutput ? "Loading full response…" : `Load full response${outputBytes ? ` (${formatBytes(outputBytes)})` : ""}`}
          </button>
          {outputError && <span class="text-accent-red">{outputError}</span>}
        </div>
      )}
    </>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.ceil(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
