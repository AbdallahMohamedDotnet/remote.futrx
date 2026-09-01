import type {
  AgentCapabilitiesCatalog,
  CapabilityPreferenceCorrection,
  CapabilityPreferenceSelection,
  ComposerCapabilityState,
  ComposerProviderOption,
} from "../../../models/agentCapabilities";
import type { ChatProvider } from "../../../models/chat";

// Resolves the composer's view of what the selected agent can do: which
// providers and models to offer, and which of the saved preferences the live
// catalog still supports.
class AgentCapabilityState {
  resolve(
    catalog: AgentCapabilitiesCatalog | null,
    provider: ChatProvider,
    model: string,
    loading: boolean,
    unavailableProviders: Partial<Record<ChatProvider, string>> = {},
  ): ComposerCapabilityState {
    const providerCapabilities = catalog?.providers.find(
      (item) => item.provider === provider,
    );
    const discoveredProviderOptions = catalog?.providers.map((item) => ({
      value: item.provider,
      label: item.label,
      disabled: !!unavailableProviders[item.provider],
      disabledReason: unavailableProviders[item.provider],
      models: item.models.map((modelItem) => ({
        value: modelItem.id,
        label: modelItem.label,
        sub: modelItem.description
          || (modelItem.providerDefault ? "provider default" : "available model"),
      })),
    })) ?? [];
    const savedProviderFallback: ComposerProviderOption = {
      value: provider,
      label: this.providerLabel(provider),
      disabled: !!unavailableProviders[provider],
      disabledReason: unavailableProviders[provider],
      models: loading ? [] : [{
        value: model,
        label: model || "Auto",
        sub: "current selection",
      }],
    };
    const providerOptions: ComposerProviderOption[] = discoveredProviderOptions.some(
      (option) => option.value === provider,
    )
      ? discoveredProviderOptions
      : [...discoveredProviderOptions, savedProviderFallback];
    const modelOptions = providerOptions.find((option) => option.value === provider)?.models ?? [];
    // A non-empty unknown selection remains a visible custom model and must
    // not inherit Auto's controls. Provider adapters may reject the stale
    // value, and silently attaching Auto-only effort/tier choices would
    // describe a command Remote does not actually launch.
    const selectedModel = providerCapabilities?.models.find((item) => item.id === model)
      ?? (model === "" ? providerCapabilities?.models.find((item) => item.id === "") : undefined);
    return {
      providerCapabilities,
      providerOptions,
      modelOptions,
      reasoningEffortOptions: selectedModel?.reasoningEfforts ?? [],
      serviceTierOptions: selectedModel?.serviceTiers ?? [],
      modeOptions: providerCapabilities?.modes ?? [],
    };
  }

  corrections(
    state: ComposerCapabilityState,
    selection: CapabilityPreferenceSelection,
  ): CapabilityPreferenceCorrection {
    const capabilities = state.providerCapabilities;
    if (!capabilities || capabilities.source !== "live") return {};

    const correction: CapabilityPreferenceCorrection = {};
    if (
      selection.reasoningEffort &&
      !state.reasoningEffortOptions.some(
        (option) => option.value === selection.reasoningEffort,
      )
    ) {
      correction.reasoningEffort = "";
    }
    if (
      selection.serviceTier &&
      !state.serviceTierOptions.some(
        (option) => option.value === selection.serviceTier,
      )
    ) {
      correction.serviceTier = "";
    }
    // Never auto-correct a mode. A queued or scheduled prompt may have been
    // composed under its read-only semantics; silently selecting Default can
    // turn a safe rejection into a later write-capable execution. The composer
    // presents an explicit switch when a saved mode is no longer available.
    return correction;
  }

  private providerLabel(provider: string): string {
    return provider ? provider.charAt(0).toUpperCase() + provider.slice(1) : "Agent";
  }
}

export const agentCapabilityState = new AgentCapabilityState();
