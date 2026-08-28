package service

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	servicechat "github.com/futrx-com/remote.futrx.com/internal/service/chat"
	servicepresence "github.com/futrx-com/remote.futrx.com/internal/service/presence"
	servicepush "github.com/futrx-com/remote.futrx.com/internal/service/push"
)

// askUserQuestionTool is the agent tool that blocks a run on a human answer.
// The frontend renders it as an inline question card; the same event is what
// makes a push notification worth sending.
const askUserQuestionTool = "AskUserQuestion"

// audienceTimeout bounds the store reads that resolve who to notify. The
// delivery itself runs separately, on the push service's own deadline.
const audienceTimeout = 5 * time.Second

// chatPushNotifier turns persisted chat events into push notifications. It
// hangs off the chat repository so every path that appends an event —
// interactive prompts, scheduled runs, recovery — is covered by construction.
type chatPushNotifier struct {
	push     *servicepush.Service
	chats    servicechat.Repository
	audience chatNotificationAudience
	presence *servicepresence.Service

	// parked records every unanswered question by native interaction ID. Legacy
	// print transports end immediately after AskUserQuestion; interactive
	// transports can have multiple non-blocking questions in flight and clear
	// each marker independently with interaction_resolved.
	mu     sync.Mutex
	parked map[servicechat.ID]map[string]struct{}
}

// ChatEvent decides whether an appended event deserves a notification and, if
// so, who receives it. It never blocks the caller on network I/O.
func (n *chatPushNotifier) ChatEvent(chatID servicechat.ID, event servicechat.Event) {
	if n == nil || !n.push.Enabled() {
		return
	}
	kind, urgent, ok := notificationKind(event)
	if !n.trackParkedRun(chatID, event, ok && kind == servicepush.KindQuestion) {
		return
	}
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), audienceTimeout)
	defer cancel()

	meta, err := n.chats.Get(ctx, chatID)
	if err != nil {
		log.Printf("push: resolve chat %s: %v", chatID, err)
		return
	}
	recipients, err := n.audience.recipients(ctx, meta)
	if err != nil {
		log.Printf("push: resolve audience for chat %s: %v", chatID, err)
		return
	}
	// Someone with this chat on screen is already watching the thing the
	// notification would announce. Dropping them here silences every device
	// they own, which the service worker cannot do: it only sees the tabs of
	// the browser it runs in, so their other phone would buzz regardless.
	recipients = n.presence.Filter(recipients, string(chatID))
	if len(recipients) == 0 {
		return
	}

	title, body := notificationText(kind, meta, event)
	n.push.NotifyAsync(recipients, servicepush.Notification{
		Kind:   kind,
		ChatID: string(chatID),
		Title:  title,
		Body:   body,
		// One tag per chat: a later notification replaces the chat's earlier
		// tray entry instead of stacking behind it.
		Tag:    "chat:" + string(chatID),
		Urgent: urgent,
	})
}

// trackParkedRun maintains the "waiting on an answer" flag and reports whether
// the event should still be allowed to notify. It swallows exactly one
// terminal event per parked run: the one the agent emits when it stops to ask.
func (n *chatPushNotifier) trackParkedRun(
	chatID servicechat.ID,
	event servicechat.Event,
	isQuestion bool,
) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.parked == nil {
		n.parked = map[servicechat.ID]map[string]struct{}{}
	}

	switch {
	case isQuestion:
		pending := n.parked[chatID]
		if pending == nil {
			pending = map[string]struct{}{}
			n.parked[chatID] = pending
		}
		pending[parkedQuestionKey(event)] = struct{}{}
		return true
	case event.Type == "interaction_resolved":
		pending := n.parked[chatID]
		delete(pending, "interaction:"+strings.TrimSpace(event.ID))
		if len(pending) == 0 {
			delete(n.parked, chatID)
		}
		return true
	case event.Type == "user":
		// A new prompt answers the question and starts a fresh run.
		delete(n.parked, chatID)
		return true
	case event.Type == "complete" || event.Type == "error":
		if len(n.parked[chatID]) > 0 {
			delete(n.parked, chatID)
			return false
		}
		return true
	default:
		return true
	}
}

func parkedQuestionKey(event servicechat.Event) string {
	prefix := "legacy:"
	if event.Type == "interaction_request" {
		prefix = "interaction:"
	}
	return prefix + strings.TrimSpace(event.ID)
}

// notificationKind maps a chat event onto a notification, or reports that the
// event is not worth interrupting anyone for. Streaming deltas, tool traffic,
// and session bookkeeping all fall through.
func notificationKind(event servicechat.Event) (kind servicepush.Kind, urgent, ok bool) {
	scheduled := strings.TrimSpace(event.ScheduledTaskID) != ""
	switch event.Type {
	case "tool_use_start":
		if event.Name != askUserQuestionTool {
			return "", false, false
		}
		// The run is now parked waiting on a human, so this one is urgent.
		return servicepush.KindQuestion, true, true
	case "interaction_request":
		if event.ToolName != askUserQuestionTool {
			return "", false, false
		}
		return servicepush.KindQuestion, true, true
	case "complete":
		if scheduled {
			return servicepush.KindScheduled, false, true
		}
		return servicepush.KindComplete, false, true
	case "error":
		if scheduled {
			return servicepush.KindScheduled, false, true
		}
		return servicepush.KindError, false, true
	default:
		return "", false, false
	}
}

func notificationText(
	kind servicepush.Kind,
	meta servicechat.Meta,
	event servicechat.Event,
) (title, body string) {
	chatTitle := strings.TrimSpace(meta.Title)
	if chatTitle == "" {
		chatTitle = "Untitled chat"
	}

	switch kind {
	case servicepush.KindQuestion:
		return "The agent is asking a question", chatTitle
	case servicepush.KindComplete:
		return "Turn finished", chatTitle
	case servicepush.KindError:
		return "Run failed", withDetail(chatTitle, event.Message)
	case servicepush.KindScheduled:
		if event.Type == "error" {
			return "Scheduled task failed", withDetail(chatTitle, event.Message)
		}
		return "Scheduled task finished", chatTitle
	default:
		return chatTitle, ""
	}
}

func withDetail(chatTitle, detail string) string {
	detail = strings.TrimSpace(strings.ReplaceAll(detail, "\n", " "))
	if detail == "" {
		return chatTitle
	}
	if len(detail) > 140 {
		detail = detail[:140] + "…"
	}
	return chatTitle + " — " + detail
}
