import type { ComponentChildren } from "preact";
import { useMemo, useState } from "preact/hooks";
import { diffLines } from "diff";
import { ChevronDown, ChevronRight, File, Edit as EditIcon, TerminalIcon, AlertCircle, Loader } from "../icons";
import { AskUserQuestion } from "./AskUserQuestion";

interface AskInput {
  questions?: Array<{
    question: string;
    header?: string;
    multiSelect?: boolean;
    options: Array<{ label: string; description?: string }>;
  }>;
}

interface ToolCallProps {
  toolUseId?: string;
  chatId?: string;
  name: string;
  input: Record<string, unknown> | undefined;
  output?: string;
  isError?: boolean;
  status: "running" | "done";
  onAnswerQuestion?: (text: string) => void;
}

export function ToolCall({ toolUseId, chatId, name, input, output, isError, status, onAnswerQuestion }: ToolCallProps) {
  // AskUserQuestion is special: render an interactive picker instead of a
  // generic tool widget. claude already auto-resolved the underlying tool in
  // -p mode, so user's pick becomes a follow-up message.
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
  // Per-tool specialized rendering.
  switch (name) {
    case "Read":
      return <ReadCall input={input} output={output} status={status} isError={isError} />;
    case "Edit":
    case "MultiEdit":
      return <EditCall input={input} output={output} status={status} isError={isError} />;
    case "Write":
      return <WriteCall input={input} output={output} status={status} isError={isError} />;
    case "Bash":
      return <BashCall input={input} output={output} status={status} isError={isError} />;
    case "Glob":
    case "Grep":
      return <SearchCall name={name} input={input} output={output} status={status} isError={isError} />;
    default:
      return <GenericCall name={name} input={input} output={output} status={status} isError={isError} />;
  }
}

// --- Shared shell ---------------------------------------------------------

function ToolShell({
  icon, label, badge, status, isError, defaultOpen, children,
}: {
  icon: ComponentChildren;
  label: ComponentChildren;
  badge?: string;
  status: "running" | "done";
  isError?: boolean;
  defaultOpen?: boolean;
  children?: ComponentChildren;
}) {
  const [open, setOpen] = useState(!!defaultOpen);
  return (
    <div class={`my-2 border rounded-lg overflow-hidden text-sm shadow-sm
                ${isError ? "border-accent-red/50 bg-accent-red/5" : "border-white/10 bg-[#101318]"}`}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        class={`w-full min-h-10 flex items-center gap-2 px-3 py-2 text-left
                ${isError ? "bg-accent-red/10" : "bg-white/[0.03] hover:bg-white/[0.06]"}`}
      >
        {children ? (open ? <ChevronDown class="w-3.5 h-3.5 text-ink-300 flex-none" /> : <ChevronRight class="w-3.5 h-3.5 text-ink-300 flex-none" />) : <span class="w-3.5 flex-none" />}
        <span class={`flex-none ${isError ? "text-accent-red" : "text-accent-blue"}`}>{icon}</span>
        <span class="flex-1 truncate text-ink-100">{label}</span>
        {badge && <span class="text-[11px] text-ink-300 flex-none">{badge}</span>}
        {status === "running"
          ? <Loader class="w-3.5 h-3.5 text-ink-300 animate-spin flex-none" />
          : isError
            ? <AlertCircle class="w-3.5 h-3.5 text-accent-red flex-none" />
            : null}
      </button>
      {open && children && (
        <div class="border-t border-white/10 bg-[#0b0d11]">
          {children}
        </div>
      )}
    </div>
  );
}

function CodeBlock({ text, lang }: { text: string; lang?: string }) {
  return (
    <pre class={`overflow-x-auto touch-scroll p-3 text-[12.5px] leading-relaxed font-mono text-ink-100
                  ${lang === "diff" ? "" : ""}`}>
      {text}
    </pre>
  );
}

function shortPath(p: string | undefined): string {
  if (!p) return "";
  if (p.startsWith("/root/")) return "~" + p.slice(5);
  return p;
}

// --- Read -----------------------------------------------------------------

function ReadCall({ input, output, status, isError }: Omit<ToolCallProps, "name">) {
  const path = (input?.file_path as string) ?? "";
  return (
    <ToolShell
      icon={<File class="w-4 h-4" />}
      label={<><span class="text-ink-300">Read</span> <span class="font-mono">{shortPath(path)}</span></>}
      status={status} isError={isError}
    >
      {output ? <CodeBlock text={truncate(output, 8000)} /> : null}
    </ToolShell>
  );
}

// --- Edit / MultiEdit -----------------------------------------------------

function EditCall({ input, output, status, isError }: Omit<ToolCallProps, "name">) {
  const path = (input?.file_path as string) ?? "";
  const oldStr = (input?.old_string as string) ?? "";
  const newStr = (input?.new_string as string) ?? "";
  const edits = (input?.edits as Array<{ old_string: string; new_string: string }>) ?? null;

  const patches = useMemo(() => {
    if (edits && Array.isArray(edits)) {
      return edits.map((e) => diffLines(e.old_string ?? "", e.new_string ?? ""));
    }
    return [diffLines(oldStr, newStr)];
  }, [oldStr, newStr, edits]);

  return (
    <ToolShell
      icon={<EditIcon class="w-4 h-4" />}
      label={<><span class="text-ink-300">Edit</span> <span class="font-mono">{shortPath(path)}</span></>}
      badge={edits ? `${edits.length} edits` : undefined}
      status={status} isError={isError}
      defaultOpen
    >
      <div class="divide-y divide-ink-500">
        {patches.map((parts, i) => (
          <pre key={i} class="overflow-x-auto touch-scroll p-3 text-[12.5px] leading-relaxed font-mono">
            {parts.map((p, j) => (
              <span
                key={j}
                class={
                  p.added ? "block bg-accent-green/15 text-accent-green" :
                  p.removed ? "block bg-accent-red/15 text-accent-red line-through" :
                  "block text-ink-200"
                }
              >
                {(p.added ? "+ " : p.removed ? "- " : "  ") + p.value.replace(/\n$/, "")}
              </span>
            ))}
          </pre>
        ))}
      </div>
      {output && !isError ? null : output ? <div class="p-3 text-accent-red font-mono text-xs">{output}</div> : null}
    </ToolShell>
  );
}

// --- Write ---------------------------------------------------------------

function WriteCall({ input, output, status, isError }: Omit<ToolCallProps, "name">) {
  const path = (input?.file_path as string) ?? "";
  const content = (input?.content as string) ?? "";
  return (
    <ToolShell
      icon={<File class="w-4 h-4" />}
      label={<><span class="text-ink-300">Write</span> <span class="font-mono">{shortPath(path)}</span></>}
      badge={`${content.split("\n").length} lines`}
      status={status} isError={isError}
    >
      <CodeBlock text={truncate(content, 8000)} />
      {output && isError ? <div class="border-t border-ink-500 p-3 text-accent-red font-mono text-xs">{output}</div> : null}
    </ToolShell>
  );
}

// --- Bash ----------------------------------------------------------------

function BashCall({ input, output, status, isError }: Omit<ToolCallProps, "name">) {
  const cmd = (input?.command as string) ?? "";
  const desc = (input?.description as string) ?? "";
  return (
    <ToolShell
      icon={<TerminalIcon class="w-4 h-4" />}
      label={
        <span class="font-mono text-[12.5px]">
          <span class="text-ink-300">$ </span>{truncate(cmd, 120)}
        </span>
      }
      badge={desc ? truncate(desc, 30) : undefined}
      status={status} isError={isError}
    >
      {output ? <CodeBlock text={truncate(output, 6000)} /> : null}
    </ToolShell>
  );
}

// --- Glob / Grep ---------------------------------------------------------

function SearchCall({ name, input, output, status, isError }: ToolCallProps) {
  const pattern = (input?.pattern as string) ?? (input?.query as string) ?? "";
  const path = (input?.path as string) ?? "";
  return (
    <ToolShell
      icon={<TerminalIcon class="w-4 h-4" />}
      label={
        <>
          <span class="text-ink-300">{name}</span>{" "}
          <span class="font-mono">{pattern}</span>
          {path && <span class="text-ink-300 ml-1">in {shortPath(path)}</span>}
        </>
      }
      status={status} isError={isError}
    >
      {output ? <CodeBlock text={truncate(output, 6000)} /> : null}
    </ToolShell>
  );
}

// --- Generic JSON fallback ------------------------------------------------

function GenericCall({ name, input, output, status, isError }: ToolCallProps) {
  return (
    <ToolShell
      icon={<TerminalIcon class="w-4 h-4" />}
      label={<span class="text-ink-300">{name}</span>}
      status={status} isError={isError}
    >
      <div class="divide-y divide-ink-500">
        {input && Object.keys(input).length > 0 && (
          <div>
            <div class="px-3 py-1 text-[11px] text-ink-300 bg-white/[0.04]">Input</div>
            <CodeBlock text={JSON.stringify(input, null, 2)} lang="json" />
          </div>
        )}
        {output && (
          <div>
            <div class="px-3 py-1 text-[11px] text-ink-300 bg-white/[0.04]">Output</div>
            <CodeBlock text={truncate(output, 6000)} />
          </div>
        )}
      </div>
    </ToolShell>
  );
}

function truncate(s: string, n: number): string {
  if (s.length <= n) return s;
  return s.slice(0, n) + `\n\n… (${s.length - n} more characters truncated)`;
}
