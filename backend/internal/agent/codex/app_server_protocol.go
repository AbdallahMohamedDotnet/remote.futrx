package codex

import (
	"encoding/json"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const (
	appServerInitializeRequestID = 1
	appServerThreadRequestID     = 2
	appServerTurnRequestID       = 3
)

type appServerEnvelope struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type appServerThreadResult struct {
	Thread struct {
		ID string `json:"id"`
	} `json:"thread"`
	Model string `json:"model"`
}

func appServerThreadRequest(req agent.RunRequest) (string, map[string]any) {
	params := map[string]any{
		"approvalPolicy": "never",
		"sandbox":        "danger-full-access",
	}
	if cwd := strings.TrimSpace(req.Cwd); cwd != "" {
		params["cwd"] = cwd
	}
	if model := sanitizeModel(req.Model); model != "" {
		params["model"] = model
	}
	if tier := serviceTierArg(req.Preferences.ServiceTier); tier != "" {
		params["serviceTier"] = tier
	}
	if req.ResumeID == "" {
		params["serviceName"] = "remote-futrx"
		return "thread/start", params
	}
	params["threadId"] = req.ResumeID
	if req.Fork {
		return "thread/fork", params
	}
	return "thread/resume", params
}

func appServerTurnParams(req agent.RunRequest, threadID, model string) map[string]any {
	mode := "default"
	if req.Mode == "plan" {
		mode = "plan"
	}
	effort := reasoningEffortArg(req.Preferences.ReasoningEffort)
	if effort == "" && mode == "plan" {
		// Codex's native Plan preset uses medium reasoning when the user has not
		// selected an explicit effort.
		effort = "medium"
	}
	settings := map[string]any{
		"model":                  model,
		"developer_instructions": nil,
		"reasoning_effort":       nullableString(effort),
	}
	params := map[string]any{
		"threadId":       threadID,
		"input":          []map[string]string{{"type": "text", "text": req.Prompt}},
		"approvalPolicy": "never",
		"sandboxPolicy":  map[string]string{"type": "dangerFullAccess"},
		"model":          model,
		"collaborationMode": map[string]any{
			"mode":     mode,
			"settings": settings,
		},
	}
	if effort != "" {
		params["effort"] = effort
	}
	if tier := serviceTierArg(req.Preferences.ServiceTier); tier != "" {
		params["serviceTier"] = tier
	}
	return params
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func rpcResponseID(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var id int
	if err := json.Unmarshal(raw, &id); err != nil {
		return 0, false
	}
	return id, true
}

func isMissingCodexThread(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "not found") || strings.Contains(lower, "no rollout")
}
