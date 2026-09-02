import { DecisionButton, RequestDetails } from "./InteractionControls";
import type { InteractionFormProps } from "./types";

export function ApprovalInteractionForm({
  method,
  input,
  disabled,
  onSubmit,
}: InteractionFormProps & { method: string }) {
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
