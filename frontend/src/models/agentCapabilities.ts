import type { ChatProvider } from "./chat";

export interface AgentCapabilityOption {
  value: string;
  label: string;
  description?: string;
}

export interface AgentModelCapability {
  id: string;
  label: string;
  description?: string;
  providerDefault?: boolean;
  reasoningEfforts: AgentCapabilityOption[];
  defaultReasoningEffort?: string;
  serviceTiers: AgentCapabilityOption[];
  defaultServiceTier?: string;
}

export interface AgentProviderCapabilities {
  provider: ChatProvider;
  label: string;
  version?: string;
  source: "live" | "fallback";
  warning?: string;
  models: AgentModelCapability[];
  modes: AgentCapabilityOption[];
  defaultMode?: string;
}

export interface AgentCapabilitiesCatalog {
  providers: AgentProviderCapabilities[];
}
