package usersettings

import (
	"context"
	"errors"
	"testing"
)

func TestDefaultSettingsUseCodexChatDefaults(t *testing.T) {
	settings := DefaultSettings()
	if settings.Chat.Provider != ChatProviderCodex {
		t.Fatalf("expected codex default provider, got %q", settings.Chat.Provider)
	}
	if settings.Chat.Mode != ChatModeCode {
		t.Fatalf("expected code default mode, got %q", settings.Chat.Mode)
	}
	if settings.Chat.Model != "" || settings.Chat.ReasoningEffort != "" {
		t.Fatalf("expected auto model and reasoning effort, got %+v", settings.Chat)
	}
}

func TestUpdatePersistsChatPreferences(t *testing.T) {
	repo := &memoryRepo{}
	service := New(repo)
	provider := ChatProviderClaude
	model := " sonnet "
	mode := ChatModePlan
	effort := ReasoningEffortHigh

	settings, err := service.Update(context.Background(), "sub:user", UpdateInput{
		Chat: &ChatUpdate{
			Provider:        &provider,
			Model:           &model,
			Mode:            &mode,
			ReasoningEffort: &effort,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Chat.Provider != ChatProviderClaude || settings.Chat.Model != "sonnet" || settings.Chat.Mode != ChatModePlan || settings.Chat.ReasoningEffort != ReasoningEffortHigh {
		t.Fatalf("unexpected chat settings: %+v", settings.Chat)
	}
	if !repo.saved {
		t.Fatal("expected settings to be saved")
	}
}

func TestUpdateRejectsInvalidChatPreferences(t *testing.T) {
	tests := []struct {
		name string
		in   UpdateInput
		want error
	}{
		{
			name: "provider",
			in: UpdateInput{Chat: &ChatUpdate{
				Provider: ptr(ChatProvider("bad")),
			}},
			want: ErrInvalidChatProvider,
		},
		{
			name: "mode",
			in: UpdateInput{Chat: &ChatUpdate{
				Mode: ptr(ChatMode("bad")),
			}},
			want: ErrInvalidChatMode,
		},
		{
			name: "reasoning effort",
			in: UpdateInput{Chat: &ChatUpdate{
				ReasoningEffort: ptr(ReasoningEffort("bad")),
			}},
			want: ErrInvalidReasoningEffort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(&memoryRepo{}).Update(context.Background(), "sub:user", tt.in)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

type memoryRepo struct {
	settings Settings
	exists   bool
	saved    bool
}

func (r *memoryRepo) Get(context.Context, Key) (Settings, error) {
	if !r.exists {
		return Settings{}, ErrNotFound
	}
	return r.settings, nil
}

func (r *memoryRepo) Save(_ context.Context, _ Key, settings Settings) (Settings, error) {
	r.settings = settings
	r.exists = true
	r.saved = true
	return settings, nil
}
