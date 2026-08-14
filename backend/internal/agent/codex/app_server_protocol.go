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
	Error  *appServerError `json:"error,omitempty"`
}

type appServerError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type appServerThreadResult struct {
	Thread appServerThread `json:"thread"`
	Model  string          `json:"model"`
}

type appServerThread struct {
	ID string `json:"id"`
}

type appServerThreadRequest struct {
	Method string
	Params appServerThreadParams
}

type appServerThreadParams struct {
	ApprovalPolicy string `json:"approvalPolicy"`
	Cwd            string `json:"cwd,omitempty"`
	Model          string `json:"model,omitempty"`
	Sandbox        string `json:"sandbox"`
	ServiceName    string `json:"serviceName,omitempty"`
	ServiceTier    string `json:"serviceTier,omitempty"`
	ThreadID       string `json:"threadId,omitempty"`
}

type appServerTurnParams struct {
	ApprovalPolicy    string                     `json:"approvalPolicy"`
	CollaborationMode appServerCollaborationMode `json:"collaborationMode"`
	Effort            string                     `json:"effort,omitempty"`
	Input             []appServerUserInput       `json:"input"`
	Model             string                     `json:"model"`
	SandboxPolicy     appServerSandboxPolicy     `json:"sandboxPolicy"`
	ServiceTier       string                     `json:"serviceTier,omitempty"`
	ThreadID          string                     `json:"threadId"`
}

type appServerCollaborationMode struct {
	Mode     string                         `json:"mode"`
	Settings appServerCollaborationSettings `json:"settings"`
}

type appServerCollaborationSettings struct {
	DeveloperInstructions *string `json:"developer_instructions"`
	Model                 string  `json:"model"`
	ReasoningEffort       *string `json:"reasoning_effort"`
}

type appServerUserInput struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type appServerSandboxPolicy struct {
	Type string `json:"type"`
}

func buildAppServerThreadRequest(req agent.RunRequest) appServerThreadRequest {
	request := appServerThreadRequest{
		Method: "thread/start",
		Params: appServerThreadParams{
			ApprovalPolicy: "never",
			Sandbox:        "danger-full-access",
			ServiceName:    "remote-futrx",
		},
	}
	if cwd := strings.TrimSpace(req.Cwd); cwd != "" {
		request.Params.Cwd = cwd
	}
	if model := sanitizeModel(req.Model); model != "" {
		request.Params.Model = model
	}
	if tier := serviceTierArg(req.Preferences.ServiceTier); tier != "" {
		request.Params.ServiceTier = tier
	}
	if req.ResumeID == "" {
		return request
	}
	request.Params.ServiceName = ""
	request.Params.ThreadID = req.ResumeID
	if req.Fork {
		request.Method = "thread/fork"
		return request
	}
	request.Method = "thread/resume"
	return request
}

func buildAppServerTurnParams(req agent.RunRequest, threadID, model string) appServerTurnParams {
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
	var reasoningEffort *string
	if effort != "" {
		reasoningEffort = &effort
	}
	params := appServerTurnParams{
		ApprovalPolicy: "never",
		CollaborationMode: appServerCollaborationMode{
			Mode: mode,
			Settings: appServerCollaborationSettings{
				Model:           model,
				ReasoningEffort: reasoningEffort,
			},
		},
		Effort:        effort,
		Input:         []appServerUserInput{{Text: req.Prompt, Type: "text"}},
		Model:         model,
		SandboxPolicy: appServerSandboxPolicy{Type: "dangerFullAccess"},
		ThreadID:      threadID,
	}
	if tier := serviceTierArg(req.Preferences.ServiceTier); tier != "" {
		params.ServiceTier = tier
	}
	return params
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
