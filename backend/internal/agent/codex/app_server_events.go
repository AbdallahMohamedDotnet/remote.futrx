package codex

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

type appServerEventParser struct {
	req       agent.RunRequest
	itemText  map[string]string
	lastUsage json.RawMessage
}

type appServerItem struct {
	ID               string          `json:"id,omitempty"`
	Type             string          `json:"type,omitempty"`
	Text             string          `json:"text,omitempty"`
	Command          string          `json:"command,omitempty"`
	AggregatedOutput string          `json:"aggregatedOutput,omitempty"`
	ExitCode         *int            `json:"exitCode,omitempty"`
	Status           string          `json:"status,omitempty"`
	Server           string          `json:"server,omitempty"`
	Tool             string          `json:"tool,omitempty"`
	Namespace        string          `json:"namespace,omitempty"`
	Arguments        json.RawMessage `json:"arguments,omitempty"`
	Result           json.RawMessage `json:"result,omitempty"`
	Error            json.RawMessage `json:"error,omitempty"`
	Changes          json.RawMessage `json:"changes,omitempty"`
	Query            string          `json:"query,omitempty"`
	Action           json.RawMessage `json:"action,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

func (item *appServerItem) UnmarshalJSON(data []byte) error {
	type alias appServerItem
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*item = appServerItem(decoded)
	item.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func newAppServerEventParser(req agent.RunRequest) *appServerEventParser {
	return &appServerEventParser{req: req, itemText: make(map[string]string)}
}

func (parser *appServerEventParser) ParseNotification(method string, raw json.RawMessage) []agent.Event {
	now := time.Now().UnixMilli()
	switch method {
	case "item/agentMessage/delta", "item/plan/delta":
		var params struct {
			ItemID string `json:"itemId"`
			Delta  string `json:"delta"`
		}
		if json.Unmarshal(raw, &params) != nil || params.Delta == "" {
			return nil
		}
		parser.itemText[params.ItemID] += params.Delta
		return []agent.Event{parser.event(now, agent.EventAssistantTextDelta, raw, func(event *agent.Event) {
			event.ItemID = params.ItemID
			event.ItemKind = agent.ItemMessage
			event.Text = params.Delta
		})}

	case "item/reasoning/summaryTextDelta", "item/reasoning/textDelta":
		var params struct {
			ItemID string `json:"itemId"`
			Delta  string `json:"delta"`
		}
		if json.Unmarshal(raw, &params) != nil || params.Delta == "" {
			return nil
		}
		return []agent.Event{parser.event(now, agent.EventReasoningDelta, raw, func(event *agent.Event) {
			event.ItemID = params.ItemID
			event.ItemKind = agent.ItemReasoning
			event.Text = params.Delta
		})}

	case "item/started", "item/completed":
		var params struct {
			Item appServerItem `json:"item"`
		}
		if json.Unmarshal(raw, &params) != nil {
			return nil
		}
		if method == "item/started" {
			return parser.itemStarted(now, raw, params.Item)
		}
		return parser.itemCompleted(now, raw, params.Item)

	case "thread/tokenUsage/updated":
		var params struct {
			TokenUsage struct {
				Last appServerTokenUsage `json:"last"`
			} `json:"tokenUsage"`
		}
		if json.Unmarshal(raw, &params) == nil {
			parser.lastUsage = params.TokenUsage.Last.normalized()
		}
		return nil

	case "turn/completed":
		var params struct {
			Turn struct {
				Status string `json:"status"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"turn"`
		}
		if json.Unmarshal(raw, &params) != nil {
			return nil
		}
		if params.Turn.Status == "failed" {
			message := "Codex turn failed"
			if params.Turn.Error != nil && strings.TrimSpace(params.Turn.Error.Message) != "" {
				message = strings.TrimSpace(params.Turn.Error.Message)
			}
			return []agent.Event{parser.event(now, agent.EventRunFailed, raw, func(event *agent.Event) {
				event.Message = message
				event.IsError = true
				event.Usage = parser.lastUsage
			})}
		}
		return []agent.Event{parser.event(now, agent.EventRunCompleted, raw, func(event *agent.Event) {
			event.Usage = parser.lastUsage
		})}

	case "error":
		var params struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &params) == nil && params.Message != "" {
			return []agent.Event{parser.event(now, agent.EventError, raw, func(event *agent.Event) {
				event.Message = params.Message
				event.IsError = true
			})}
		}
	}
	return nil
}

func (parser *appServerEventParser) itemStarted(
	now int64,
	raw json.RawMessage,
	item appServerItem,
) []agent.Event {
	switch item.Type {
	case "commandExecution":
		return []agent.Event{parser.toolStarted(now, raw, item.ID, "Bash", mustJSON(map[string]any{
			"command": item.Command,
		}))}
	case "fileChange":
		return []agent.Event{parser.toolStarted(now, raw, item.ID, "Patch", mustJSON(map[string]any{
			"changes": item.Changes,
		}))}
	case "mcpToolCall":
		return []agent.Event{parser.toolStarted(now, raw, item.ID, appServerMCPToolName(item), item.Arguments)}
	case "dynamicToolCall":
		return []agent.Event{parser.toolStarted(now, raw, item.ID, joinedToolName(item.Namespace, item.Tool), item.Arguments)}
	case "collabAgentToolCall":
		return []agent.Event{parser.toolStarted(now, raw, item.ID, "Collab:"+item.Tool, item.Raw)}
	case "webSearch":
		return []agent.Event{parser.toolStarted(now, raw, item.ID, "WebSearch", mustJSON(map[string]any{
			"query": item.Query, "action": item.Action,
		}))}
	default:
		return nil
	}
}

func (parser *appServerEventParser) itemCompleted(
	now int64,
	raw json.RawMessage,
	item appServerItem,
) []agent.Event {
	switch item.Type {
	case "agentMessage", "plan":
		return parser.completedText(now, raw, item)
	case "commandExecution":
		return []agent.Event{parser.toolCompleted(
			now, raw, item.ID, item.AggregatedOutput, appServerItemFailed(item),
		)}
	case "fileChange":
		return []agent.Event{parser.toolCompleted(now, raw, item.ID, item.Status, appServerItemFailed(item))}
	case "mcpToolCall", "dynamicToolCall":
		return []agent.Event{parser.toolCompleted(now, raw, item.ID, appServerToolOutput(item), appServerItemFailed(item))}
	case "collabAgentToolCall":
		return []agent.Event{parser.toolCompleted(now, raw, item.ID, item.Status, appServerItemFailed(item))}
	case "webSearch":
		return []agent.Event{parser.toolCompleted(now, raw, item.ID, item.Query, false)}
	default:
		return nil
	}
}

func (parser *appServerEventParser) completedText(
	now int64,
	raw json.RawMessage,
	item appServerItem,
) []agent.Event {
	if item.Text == "" {
		return nil
	}
	seen := parser.itemText[item.ID]
	delta := item.Text
	if strings.HasPrefix(item.Text, seen) {
		delta = item.Text[len(seen):]
	}
	parser.itemText[item.ID] = item.Text
	if delta == "" {
		return nil
	}
	return []agent.Event{parser.event(now, agent.EventAssistantTextDelta, raw, func(event *agent.Event) {
		event.ItemID = item.ID
		event.ItemKind = agent.ItemMessage
		event.Text = delta
	})}
}

func (parser *appServerEventParser) toolStarted(
	now int64,
	raw json.RawMessage,
	id, name string,
	input json.RawMessage,
) agent.Event {
	return parser.event(now, agent.EventToolStarted, raw, func(event *agent.Event) {
		event.ItemID = id
		event.ItemKind = agent.ItemToolCall
		event.ToolName = strings.TrimSpace(name)
		if event.ToolName == "" {
			event.ToolName = "CodexTool"
		}
		event.Input = input
	})
}

func (parser *appServerEventParser) toolCompleted(
	now int64,
	raw json.RawMessage,
	id, output string,
	isError bool,
) agent.Event {
	return parser.event(now, agent.EventToolCompleted, raw, func(event *agent.Event) {
		event.ItemID = id
		event.ItemKind = agent.ItemToolCall
		event.Output = output
		event.IsError = isError
	})
}

func (parser *appServerEventParser) event(
	now int64,
	eventType agent.EventType,
	raw json.RawMessage,
	configure func(*agent.Event),
) agent.Event {
	event := agent.Event{
		T:              now,
		Type:           eventType,
		Provider:       agent.ProviderCodex,
		ConversationID: parser.req.ConversationID,
		Raw:            append(json.RawMessage(nil), raw...),
	}
	if configure != nil {
		configure(&event)
	}
	return event
}

type appServerTokenUsage struct {
	InputTokens           int64 `json:"inputTokens"`
	CachedInputTokens     int64 `json:"cachedInputTokens"`
	OutputTokens          int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
}

func (usage appServerTokenUsage) normalized() json.RawMessage {
	data, _ := json.Marshal(map[string]int64{
		"input_tokens":            usage.InputTokens,
		"cache_read_input_tokens": usage.CachedInputTokens,
		"output_tokens":           usage.OutputTokens,
		"reasoning_output_tokens": usage.ReasoningOutputTokens,
	})
	return data
}

func appServerMCPToolName(item appServerItem) string {
	return joinedToolName(item.Server, item.Tool)
}

func joinedToolName(namespace, tool string) string {
	if namespace == "" {
		return tool
	}
	if tool == "" {
		return namespace
	}
	return namespace + "/" + tool
}

func appServerItemFailed(item appServerItem) bool {
	if item.ExitCode != nil && *item.ExitCode != 0 {
		return true
	}
	status := strings.ToLower(item.Status)
	return status == "failed" || status == "declined" || status == "denied"
}

func appServerToolOutput(item appServerItem) string {
	if len(item.Error) > 0 && string(item.Error) != "null" {
		return compactJSON(item.Error)
	}
	if len(item.Result) > 0 && string(item.Result) != "null" {
		return compactJSON(item.Result)
	}
	return item.Status
}

func compactJSON(raw json.RawMessage) string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return string(raw)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}
