package prompt

import (
	"context"
	"log"
	"strings"
	"time"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
)

// promptRunSnapshot contains the durable state captured while a chat's run
// reservation is held. Keeping it separate from StartInput prevents callers
// from carrying service-owned execution state into a new prompt.
type promptRunSnapshot struct {
	meta                ChatMeta
	priorEvents         []ChatEvent
	priorEventsLoaded   bool
	userPromptPersisted bool
}

type promptAdmission struct {
	snapshot  promptRunSnapshot
	duplicate bool
}

// admitPrompt snapshots the chat and durably accepts a user turn while the
// run hub holds the chat's transition barrier. A duplicate delivery is
// recognized without appending or launching another provider run.
func (rnr *Service) admitPrompt(
	ctx context.Context,
	input StartInput,
	emitTransient func(ChatEvent),
) (promptAdmission, error) {
	meta, err := rnr.store.Get(ctx, input.ChatID)
	if err != nil {
		return promptAdmission{}, err
	}
	priorEvents, err := rnr.store.ReadEvents(ctx, input.ChatID)
	if err != nil {
		return promptAdmission{}, err
	}

	admission := promptAdmission{snapshot: promptRunSnapshot{
		meta:              meta,
		priorEvents:       priorEvents,
		priorEventsLoaded: true,
	}}
	if input.ClientID != "" {
		switch meta.PromptReceipts.Status(input.ClientID, input.Prompt) {
		case servicechat.PromptReceiptConflict:
			return promptAdmission{}, ErrPromptClientIDConflict
		case servicechat.PromptReceiptAccepted:
			admission.duplicate = true
			return admission, nil
		default:
			if priorAccepted, found := acceptedPromptForClientID(priorEvents, input.ClientID); found {
				if priorAccepted.Text != input.Prompt {
					return promptAdmission{}, ErrPromptClientIDConflict
				}
				if err := rnr.storePromptReceipt(
					ctx,
					input.ChatID,
					input.ClientID,
					input.Prompt,
				); err != nil {
					log.Printf("chat %s prompt receipt migration: %v", input.ChatID, err)
				}
				admission.duplicate = true
				return admission, nil
			}
		}
	}

	if err := rnr.validateExecutionPreferences(meta, input.Expected, emitTransient); err != nil {
		return promptAdmission{}, err
	}
	userEvent := ChatEvent{
		T:               time.Now().UnixMilli(),
		Type:            "user",
		Text:            input.Prompt,
		ScheduledTaskID: input.ScheduledTaskID,
	}
	if input.ClientID != "" {
		userEvent.SetPromptClientID(input.ClientID)
	}
	storedUserEvent, err := rnr.store.AppendEvent(ctx, input.ChatID, userEvent)
	if err != nil {
		return promptAdmission{}, err
	}
	if input.ClientID != "" {
		if err := rnr.storePromptReceipt(
			ctx,
			input.ChatID,
			input.ClientID,
			input.Prompt,
		); err != nil {
			// The user event is already durable and makes reconnect retries
			// idempotent. Rewind also backfills its hidden receipt before it
			// can remove that event, so this remains an accepted prompt.
			log.Printf("chat %s prompt receipt commit: %v", input.ChatID, err)
		}
	}
	rnr.hub.BroadcastTransient(input.ChatID, storedUserEvent)
	admission.snapshot.userPromptPersisted = true
	return admission, nil
}

func acceptedPromptForClientID(events []ChatEvent, clientID string) (ChatEvent, bool) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return ChatEvent{}, false
	}
	for _, event := range events {
		if event.Type != "user" || len(event.Data) == 0 {
			continue
		}
		if event.PromptClientID() == clientID {
			return event, true
		}
	}
	return ChatEvent{}, false
}

func (rnr *Service) storePromptReceipt(
	ctx context.Context,
	chatID servicechat.ID,
	clientID string,
	prompt string,
) error {
	_, err := rnr.store.Update(ctx, chatID, func(meta *ChatMeta) {
		meta.PromptReceipts = meta.PromptReceipts.WithAcceptedPrompt(clientID, prompt)
	})
	return err
}
