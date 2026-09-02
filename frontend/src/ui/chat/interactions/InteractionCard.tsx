import { useState } from "preact/hooks";
import type { AssistantMessagePart } from "../../../models/chatMessage";

type InteractionPart = Extract<AssistantMessagePart, { kind: "interaction" }>;

interface UserQuestion {
  id?: string;
  header?: string;
  question?: string;
  options?: Array<{ label?: string; description?: string }>;
  isOther?: boolean;
  isSecret?: boolean;
}

export function InteractionCard({
  part,
  onRespond,
}: {
  part: InteractionPart;
  onRespond?: (interactionId: string, result?: unknown, error?: unknown) => boolean;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState("");

  function respond(result?: unknown, responseError?: unknown) {
    if (!onRespond || submitting || part.status !== "pending") return;
    if (!onRespond(part.id, result, responseError)) {
      setLocalError("The interaction response could not be sent. Check the connection and retry.");
      return;
    }
    setLocalError("");
    setSubmitting(true);
  }

  return (
    <section class="my-2 overflow-hidden rounded-lg border border-accent-blue/35 bg-accent-blue/[0.05]">
      <header class="flex items-center justify-between gap-3 border-b border-accent-blue/20 bg-accent-blue/[0.08] px-3 py-2">
        <div class="min-w-0">
          <div class="text-[11px] font-semibold text-accent-blue">Codex needs your decision</div>
          <div class="truncate font-mono text-[10px] text-ink-400">{part.method}</div>
        </div>
        <span class="rounded-full border border-line px-2 py-0.5 text-[10px] text-ink-300">
          {submitting && part.status === "pending" ? "sending" : part.status}
        </span>
      </header>
      <div class="p-3">
        {part.status !== "pending" ? (
          <p class="text-[12px] text-ink-300">Request {humanStatus(part.status)}.</p>
        ) : part.interactionKind === "user_input" ? (
          <UserInputForm input={part.input} disabled={submitting} onSubmit={respond} />
        ) : part.interactionKind === "approval" ? (
          <ApprovalForm method={part.method} input={part.input} disabled={submitting} onSubmit={respond} />
        ) : part.interactionKind === "permission" ? (
          <PermissionForm input={part.input} disabled={submitting} onSubmit={respond} />
        ) : part.interactionKind === "elicitation" ? (
          <ElicitationForm input={part.input} disabled={submitting} onSubmit={respond} />
        ) : (
          <GenericRequestForm input={part.input} disabled={submitting} onSubmit={respond} />
        )}
        {localError && <p class="mt-2 text-[11px] text-accent-red">{localError}</p>}
      </div>
    </section>
  );
}

function UserInputForm({
  input,
  disabled,
  onSubmit,
}: {
  input: Record<string, unknown>;
  disabled: boolean;
  onSubmit: (result: unknown) => void;
}) {
  const questions = Array.isArray(input.questions) ? input.questions as UserQuestion[] : [];
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const complete = questions.length > 0 && questions.every((question, index) => {
    const id = question.id || String(index);
    return (answers[id] || "").trim().length > 0;
  });

  function submit() {
    const encoded: Record<string, { answers: string[] }> = {};
    questions.forEach((question, index) => {
      const id = question.id || String(index);
      encoded[id] = { answers: [(answers[id] || "").trim()] };
    });
    onSubmit({ answers: encoded });
  }

  return (
    <div class="space-y-4">
	  {typeof input.autoResolutionMs === "number" && (
		<p class="text-[10px] text-ink-400">
		  Codex may auto-resolve this request after {Math.ceil(input.autoResolutionMs / 1000)} seconds.
		</p>
	  )}
      {questions.map((question, index) => {
        const id = question.id || String(index);
        const options = Array.isArray(question.options) ? question.options : [];
        return (
          <fieldset key={id} class="space-y-2" disabled={disabled}>
            <legend class="text-[13px] font-medium leading-snug text-ink-100">
              {question.header && <span class="mr-2 font-mono text-[10px] text-ink-400">{question.header}</span>}
              {question.question || "Codex is requesting input"}
            </legend>
            {options.length > 0 && (
              <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
                {options.map((option, optionIndex) => (
                  <button
                    key={`${id}-${optionIndex}`}
                    type="button"
                    onClick={() => setAnswers((current) => ({ ...current, [id]: option.label || "" }))}
                    class={`rounded-control border px-2.5 py-2 text-left text-[12px] transition ${answers[id] === option.label ? "border-accent-blue bg-accent-blue/10 text-ink-100" : "border-line bg-surface text-ink-200 hover:border-line-strong"}`}
                  >
                    <span class="block font-medium">{option.label}</span>
                    {option.description && <span class="mt-0.5 block text-[10px] text-ink-400">{option.description}</span>}
                  </button>
                ))}
              </div>
            )}
            {(options.length === 0 || question.isOther) && (
              <input
                type={question.isSecret ? "password" : "text"}
                value={answers[id] || ""}
                autocomplete="off"
                placeholder={question.isSecret ? "Secret answer (not saved to chat history)" : "Type an answer"}
                onInput={(event) => setAnswers((current) => ({
                  ...current,
                  [id]: (event.currentTarget as HTMLInputElement).value,
                }))}
                class="h-9 w-full rounded-control border border-line bg-canvas px-2.5 text-[12px] text-ink-100 outline-none focus:border-accent-blue"
              />
            )}
          </fieldset>
        );
      })}
      <DecisionButton disabled={disabled || !complete} onClick={submit}>Send answers</DecisionButton>
    </div>
  );
}

function ApprovalForm({
  method,
  input,
  disabled,
  onSubmit,
}: {
  method: string;
  input: Record<string, unknown>;
  disabled: boolean;
  onSubmit: (result: unknown) => void;
}) {
  const legacy = method === "execCommandApproval" || method === "applyPatchApproval";
  return (
    <div class="space-y-3">
      <RequestDetails input={input} />
      <div class="flex flex-wrap gap-2">
        <DecisionButton disabled={disabled} onClick={() => onSubmit({ decision: legacy ? "approved" : "accept" })}>
          Allow once
        </DecisionButton>
        <DecisionButton disabled={disabled} onClick={() => onSubmit({ decision: legacy ? "approved_for_session" : "acceptForSession" })}>
          Allow for session
        </DecisionButton>
        <DecisionButton tone="danger" disabled={disabled} onClick={() => onSubmit({
          decision: legacy ? { denied: { rejection: "Denied by user" } } : "decline",
        })}>
          Deny
        </DecisionButton>
        {!legacy && (
          <DecisionButton disabled={disabled} onClick={() => onSubmit({ decision: "cancel" })}>Cancel request</DecisionButton>
        )}
      </div>
      <p class="text-[10px] text-ink-400">“Allow for session” applies to matching requests in this Codex session.</p>
    </div>
  );
}

function PermissionForm({
  input,
  disabled,
  onSubmit,
}: {
  input: Record<string, unknown>;
  disabled: boolean;
  onSubmit: (result: unknown) => void;
}) {
  const permissions = isObject(input.permissions) ? input.permissions : {};
  return (
    <div class="space-y-3">
      {typeof input.reason === "string" && <p class="text-[13px] text-ink-200">{input.reason}</p>}
      <RequestDetails input={input} />
      <div class="flex flex-wrap gap-2">
        <DecisionButton disabled={disabled} onClick={() => onSubmit({ permissions, scope: "turn" })}>Grant for turn</DecisionButton>
        <DecisionButton disabled={disabled} onClick={() => onSubmit({ permissions, scope: "session" })}>Grant for session</DecisionButton>
        <DecisionButton tone="danger" disabled={disabled} onClick={() => onSubmit({ permissions: {}, scope: "turn" })}>Deny</DecisionButton>
      </div>
    </div>
  );
}

function ElicitationForm({
  input,
  disabled,
  onSubmit,
}: {
  input: Record<string, unknown>;
  disabled: boolean;
  onSubmit: (result: unknown) => void;
}) {
  const [content, setContent] = useState("{}");
  const [error, setError] = useState("");
  function accept() {
    try {
      onSubmit({ action: "accept", content: JSON.parse(content) });
    } catch {
      setError("Enter valid JSON content.");
    }
  }
  return (
    <div class="space-y-3">
      <RequestDetails input={input} />
      <textarea
        value={content}
        disabled={disabled}
        onInput={(event) => setContent((event.currentTarget as HTMLTextAreaElement).value)}
        class="min-h-24 w-full rounded-control border border-line bg-canvas p-2 font-mono text-[11px] text-ink-100 outline-none focus:border-accent-blue"
        aria-label="Elicitation response JSON"
      />
      {error && <p class="text-[11px] text-accent-red">{error}</p>}
      <div class="flex flex-wrap gap-2">
        <DecisionButton disabled={disabled} onClick={accept}>Accept</DecisionButton>
        <DecisionButton tone="danger" disabled={disabled} onClick={() => onSubmit({ action: "decline" })}>Decline</DecisionButton>
        <DecisionButton disabled={disabled} onClick={() => onSubmit({ action: "cancel" })}>Cancel</DecisionButton>
      </div>
    </div>
  );
}

function GenericRequestForm({
  input,
  disabled,
  onSubmit,
}: {
  input: Record<string, unknown>;
  disabled: boolean;
  onSubmit: (result?: unknown, error?: unknown) => void;
}) {
  const [result, setResult] = useState("null");
  const [error, setError] = useState("");
  function submit() {
    try {
      onSubmit(JSON.parse(result));
    } catch {
      setError("Enter a valid JSON result.");
    }
  }
  return (
    <div class="space-y-3">
      <p class="text-[12px] text-ink-300">This provider request has no dedicated Remote form. Review it and return an explicit JSON result.</p>
      <RequestDetails input={input} />
      <textarea
        value={result}
        disabled={disabled}
        onInput={(event) => setResult((event.currentTarget as HTMLTextAreaElement).value)}
        class="min-h-20 w-full rounded-control border border-line bg-canvas p-2 font-mono text-[11px] text-ink-100 outline-none focus:border-accent-blue"
        aria-label="Provider request result JSON"
      />
      {error && <p class="text-[11px] text-accent-red">{error}</p>}
      <div class="flex flex-wrap gap-2">
        <DecisionButton disabled={disabled} onClick={submit}>Send result</DecisionButton>
        <DecisionButton tone="danger" disabled={disabled} onClick={() => onSubmit(undefined, {
          code: -32601,
          message: "Unsupported provider request declined by user",
        })}>
          Decline as unsupported
        </DecisionButton>
      </div>
    </div>
  );
}

function RequestDetails({ input }: { input: Record<string, unknown> }) {
  return (
    <details class="rounded-control border border-line bg-canvas px-2.5 py-2">
      <summary class="cursor-pointer text-[11px] font-medium text-ink-300">Request details and scope</summary>
      <pre class="mt-2 max-h-56 overflow-auto whitespace-pre-wrap break-all font-mono text-[10px] leading-relaxed text-ink-400">
        {JSON.stringify(input, null, 2)}
      </pre>
    </details>
  );
}

function DecisionButton({
  children,
  disabled,
  tone = "normal",
  onClick,
}: {
  children: string;
  disabled: boolean;
  tone?: "normal" | "danger";
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      class={`h-8 rounded-control border px-3 text-[11px] font-medium transition disabled:cursor-not-allowed disabled:opacity-50 ${tone === "danger" ? "border-accent-red/40 text-accent-red hover:bg-accent-red/10" : "border-line-strong text-ink-200 hover:bg-tint-strong"}`}
    >
      {children}
    </button>
  );
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function humanStatus(status: string): string {
  return status.replaceAll("_", " ");
}
