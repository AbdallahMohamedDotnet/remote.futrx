import type {
  ChatInteractionIntent,
  ChatInteractionWireResponse,
} from "../../models/chatInteraction";

const LEGACY_APPROVAL_METHODS = new Set([
  "execCommandApproval",
  "applyPatchApproval",
]);

export function encodeInteractionResponse(
  method: string,
  intent: ChatInteractionIntent
): ChatInteractionWireResponse {
  const legacyApproval = LEGACY_APPROVAL_METHODS.has(method);

  switch (intent.kind) {
    case "answer_questions":
      return {
        result: {
          answers: Object.fromEntries(
            Object.entries(intent.answers).map(([id, answers]) => [id, { answers }])
          ),
        },
      };
    case "approve":
      return {
        result: {
          decision: legacyApproval
            ? intent.scope === "session"
              ? "approved_for_session"
              : "approved"
            : intent.scope === "session"
              ? "acceptForSession"
              : "accept",
        },
      };
    case "deny_approval":
      return {
        result: {
          decision: legacyApproval
            ? { denied: { rejection: "Denied by user" } }
            : "decline",
        },
      };
    case "cancel_approval":
      return { result: { decision: "cancel" } };
    case "grant_permissions":
      return {
        result: {
          permissions: intent.permissions,
          scope: intent.scope,
        },
      };
    case "deny_permissions":
      return { result: { permissions: {}, scope: "turn" } };
    case "accept_elicitation":
      return { result: { action: "accept", content: intent.content } };
    case "decline_elicitation":
      return { result: { action: "decline" } };
    case "cancel_elicitation":
      return { result: { action: "cancel" } };
    case "submit_provider_result":
      return { result: intent.result };
    case "decline_unsupported":
      return {
        error: {
          code: -32601,
          message: "Unsupported provider request declined by user",
        },
      };
  }
}
