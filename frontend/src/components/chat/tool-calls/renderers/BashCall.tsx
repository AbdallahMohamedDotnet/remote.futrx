import { TerminalIcon } from "../../../ui/icons";
import type { ToolCallProps } from "../ToolCallTypes";
import { CodeBlock } from "../CodeBlock";
import { ToolShell } from "../ToolShell";
import { truncate } from "../utils";

export function BashCall({ output, status, isError }: Omit<ToolCallProps, "name">) {
  return (
    <ToolShell
      icon={<TerminalIcon class="w-4 h-4" />}
      label={<span class="font-medium">Tool Use</span>}
      status={status}
      isError={isError}
    >
      {output ? <CodeBlock text={truncate(output, 6000)} /> : null}
    </ToolShell>
  );
}
