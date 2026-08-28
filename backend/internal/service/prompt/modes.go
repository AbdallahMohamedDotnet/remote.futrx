package prompt

import (
	"fmt"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const unsupportedRunModeMessage = "The saved mode is not available for this agent transport. Choose a supported mode before sending again; nothing was run."
const executionPreferencesChangedMessage = "The agent or mode changed after this prompt was composed. Review the current controls and send it again; nothing was run."

func (rnr *Service) validateExecutionPreferences(
	meta ChatMeta,
	expected *ExecutionPreferences,
	emitTransient func(ChatEvent),
) error {
	mode := normalizedRunMode(meta.Mode)
	provider := providerIDFromChatProvider(meta.Provider)
	if expected != nil {
		expectedProvider := agent.NormalizeProviderID(string(expected.Provider))
		expectedMode := normalizedRunMode(expected.Mode)
		if expectedProvider == "" || strings.TrimSpace(expected.Mode) == "" ||
			expectedProvider != provider || expectedMode != mode {
			emitTransient(ChatEvent{
				T: time.Now().UnixMilli(), Type: "error", Message: executionPreferencesChangedMessage,
			})
			return ErrExecutionPreferencesChanged
		}
	}
	if rnr.supportsRunMode(provider, mode) {
		return nil
	}
	emitTransient(ChatEvent{
		T:       time.Now().UnixMilli(),
		Type:    "error",
		Message: unsupportedRunModeMessage,
	})
	return fmt.Errorf("%w: %s", agent.ErrUnsupportedRunMode, mode)
}

func (rnr *Service) supportsRunMode(provider agent.ProviderID, mode agent.RunMode) bool {
	if mode != agent.RunModeDefault && mode != agent.RunModePlan {
		return false
	}
	if rnr.agentPolicy == nil {
		return mode == agent.RunModeDefault
	}
	return rnr.agentPolicy.SupportsRunMode(string(provider), mode)
}

func normalizedRunMode(value string) agent.RunMode {
	mode := agent.RunMode(strings.ToLower(strings.TrimSpace(value)))
	if mode == "" {
		return agent.RunModeDefault
	}
	return mode
}
