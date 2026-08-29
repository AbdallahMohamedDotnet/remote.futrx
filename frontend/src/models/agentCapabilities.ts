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
  default?: boolean;
  executionScopes?: Array<"host" | "project">;
  authentication?: {
    mode: "managed-code" | "managed-device" | "external" | "none";
    instructions?: string;
    satisfiesAccessGate: boolean;
  };
  features?: {
    sessions: { resume: boolean; fork: boolean };
    skills: "none" | "slash-command" | "dollar-mention" | "instructions";
    browserTools: boolean;
    scheduledTools: boolean;
  };
  version?: string;
  source: "live" | "fallback";
  warning?: string;
  unavailableReason?: string;
  models: AgentModelCapability[];
  modes: AgentCapabilityOption[];
  defaultMode?: string;
}

export interface AgentCapabilitiesCatalog {
  providers: AgentProviderCapabilities[];
}

/** What one browser currently knows about a catalog scope. */
export interface AgentCapabilityCatalogSnapshot {
  catalog: AgentCapabilitiesCatalog | null;
  loading: boolean;
  refreshing: boolean;
  error: string;
}

export interface ComposerModelOption {
  value: string;
  label: string;
  sub: string;
}

export interface ComposerProviderOption {
  value: ChatProvider;
  label: string;
  disabled?: boolean;
  disabledReason?: string;
  models: ComposerModelOption[];
}

/** The composer's view of what the selected agent can do: which providers
 *  and models to offer, and which options the selected model supports. */
export interface ComposerCapabilityState {
  providerCapabilities?: AgentProviderCapabilities;
  providerOptions: ComposerProviderOption[];
  modelOptions: ComposerModelOption[];
  reasoningEffortOptions: AgentCapabilityOption[];
  serviceTierOptions: AgentCapabilityOption[];
  modeOptions: AgentCapabilityOption[];
}

export interface CapabilityPreferenceSelection {
  mode: string;
  reasoningEffort: string;
  serviceTier: string;
}

export interface CapabilityPreferenceCorrection {
  mode?: string;
  reasoningEffort?: string;
  serviceTier?: string;
}
