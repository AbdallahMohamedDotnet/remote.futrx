import type { ChatProvider } from "../../../models/chat";
import { agentCapabilityState } from "../../chat/agentCapabilityState";
import { useAgentCapabilities } from "./useAgentCapabilities";

export function useComposerAgentCapabilities({
  projectId,
  provider,
  model,
}: {
  projectId?: string;
  provider: ChatProvider;
  model: string;
}) {
  const capabilities = useAgentCapabilities(projectId);
  return agentCapabilityState.resolve(
    capabilities.catalog,
    provider,
    model,
    capabilities.loading,
  );
}
