package codexharness

import (
	"encoding/json"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const (
	appServerInitializeRequestID = 1
	appServerThreadRequestID     = 2
	appServerTurnRequestID       = 3
	appServerInterruptRequestID  = 4
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
	Mode     agent.RunMode                  `json:"mode"`
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

type appServerItem struct {
	ID                string                         `json:"id,omitempty"`
	Type              string                         `json:"type,omitempty"`
	Text              string                         `json:"text,omitempty"`
	Command           string                         `json:"command,omitempty"`
	AggregatedOutput  string                         `json:"aggregatedOutput,omitempty"`
	ExitCode          *int                           `json:"exitCode,omitempty"`
	Status            string                         `json:"status,omitempty"`
	Server            string                         `json:"server,omitempty"`
	Tool              string                         `json:"tool,omitempty"`
	Namespace         string                         `json:"namespace,omitempty"`
	Arguments         json.RawMessage                `json:"arguments,omitempty"`
	Result            json.RawMessage                `json:"result,omitempty"`
	Error             json.RawMessage                `json:"error,omitempty"`
	Changes           json.RawMessage                `json:"changes,omitempty"`
	Query             string                         `json:"query,omitempty"`
	Action            json.RawMessage                `json:"action,omitempty"`
	SenderThreadID    string                         `json:"senderThreadId,omitempty"`
	ReceiverThreadIDs []string                       `json:"receiverThreadIds,omitempty"`
	AgentsStates      map[string]appServerAgentState `json:"agentsStates,omitempty"`
	Prompt            *string                        `json:"prompt,omitempty"`
	Model             *string                        `json:"model,omitempty"`
	ReasoningEffort   *string                        `json:"reasoningEffort,omitempty"`
	Raw               json.RawMessage                `json:"-"`
}

type appServerAgentState struct {
	Status  string  `json:"status"`
	Message *string `json:"message,omitempty"`
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

type appServerDeltaParams struct {
	ItemID string `json:"itemId"`
	Delta  string `json:"delta"`
}

type appServerItemParams struct {
	Item appServerItem `json:"item"`
}

type appServerTokenUsageParams struct {
	TokenUsage appServerTokenUsageSnapshot `json:"tokenUsage"`
}

type appServerTokenUsageSnapshot struct {
	Last appServerTokenUsage `json:"last"`
}

type appServerTokenUsage struct {
	InputTokens           int64 `json:"inputTokens"`
	CachedInputTokens     int64 `json:"cachedInputTokens"`
	CacheWriteInputTokens int64 `json:"cacheWriteInputTokens"`
	OutputTokens          int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
}

type appServerTurnCompletedParams struct {
	Turn appServerTurnResult `json:"turn"`
}

type appServerThreadStatusParams struct {
	ThreadID string `json:"threadId"`
	Status   struct {
		Type        string   `json:"type"`
		ActiveFlags []string `json:"activeFlags,omitempty"`
	} `json:"status"`
}

type appServerRequestResolvedParams struct {
	ThreadID  string          `json:"threadId"`
	RequestID json.RawMessage `json:"requestId"`
}

type appServerTurnResult struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Error  *appServerError `json:"error"`
}

type appServerErrorParams struct {
	Error     appServerErrorDetail `json:"error"`
	WillRetry bool                 `json:"willRetry,omitempty"`
}

type appServerErrorDetail struct {
	Message           string          `json:"message"`
	CodexErrorInfo    json.RawMessage `json:"codexErrorInfo,omitempty"`
	AdditionalDetails json.RawMessage `json:"additionalDetails,omitempty"`
}

type appServerUserInputRequestParams struct {
	ThreadID         string                  `json:"threadId"`
	TurnID           string                  `json:"turnId"`
	ItemID           string                  `json:"itemId"`
	Questions        []appServerUserQuestion `json:"questions"`
	IsBlocking       bool                    `json:"isBlocking"`
	AutoResolutionMs *uint64                 `json:"autoResolutionMs"`
}

type appServerUserQuestion struct {
	Header   string                    `json:"header"`
	ID       string                    `json:"id"`
	Question string                    `json:"question"`
	Options  []appServerQuestionOption `json:"options"`
	IsOther  bool                      `json:"isOther"`
	IsSecret bool                      `json:"isSecret"`
}

type appServerQuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

func buildAppServerThreadRequest(req agent.RunRequest) appServerThreadRequest {
	approvalPolicy := agent.NormalizeApprovalPolicy(req.Preferences.ApprovalPolicy)
	sandboxPolicy := agent.NormalizeSandboxPolicy(req.Preferences.SandboxPolicy)
	request := appServerThreadRequest{
		Method: "thread/start",
		Params: appServerThreadParams{
			ApprovalPolicy: approvalPolicy,
			Sandbox:        legacySandboxMode(sandboxPolicy),
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
	mode := agent.RunModeDefault
	if req.Mode == agent.RunModePlan {
		mode = agent.RunModePlan
	}
	effort := reasoningEffortArg(req.Preferences.ReasoningEffort)
	var reasoningEffort *string
	if effort != "" {
		reasoningEffort = &effort
	}
	params := appServerTurnParams{
		ApprovalPolicy: agent.NormalizeApprovalPolicy(req.Preferences.ApprovalPolicy),
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
		SandboxPolicy: appServerSandboxPolicy{Type: agent.NormalizeSandboxPolicy(req.Preferences.SandboxPolicy)},
		ThreadID:      threadID,
	}
	if tier := serviceTierArg(req.Preferences.ServiceTier); tier != "" {
		params.ServiceTier = tier
	}
	return params
}

func legacySandboxMode(policy string) string {
	switch agent.NormalizeSandboxPolicy(policy) {
	case "readOnly":
		return "read-only"
	case "dangerFullAccess":
		return "danger-full-access"
	default:
		return "workspace-write"
	}
}

func sanitizeModel(model string) string {
	model = strings.TrimSpace(model)
	if idx := strings.Index(model, "["); idx > 0 {
		model = strings.TrimSpace(model[:idx])
	}
	return model
}

func reasoningEffortArg(effort agent.ReasoningEffort) string {
	return agent.NormalizeCapabilityValue(string(effort))
}

func serviceTierArg(tier agent.ServiceTier) string {
	return agent.NormalizeCapabilityValue(string(tier))
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

func isMissingThread(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "not found") || strings.Contains(lower, "no rollout")
}
