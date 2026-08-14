package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const capabilityTimeout = 12 * time.Second

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type modelListResponse struct {
	Data []modelListItem `json:"data"`
}

type modelListItem struct {
	ID                        string                `json:"id"`
	Model                     string                `json:"model"`
	DisplayName               string                `json:"displayName"`
	Description               string                `json:"description"`
	DefaultReasoningEffort    string                `json:"defaultReasoningEffort"`
	DefaultServiceTier        string                `json:"defaultServiceTier"`
	IsDefault                 bool                  `json:"isDefault"`
	SupportedReasoningEfforts []reasoningEffortItem `json:"supportedReasoningEfforts"`
	ServiceTiers              []serviceTierItem     `json:"serviceTiers"`
}

type reasoningEffortItem struct {
	ReasoningEffort string `json:"reasoningEffort"`
	Description     string `json:"description"`
}

type serviceTierItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type collaborationModeListResponse struct {
	Data []collaborationModeItem `json:"data"`
}

type collaborationModeItem struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

func (p *Provider) Capabilities(ctx context.Context, req agent.CapabilityRequest) (agent.Capabilities, error) {
	appServerCtx, cancelAppServer := context.WithTimeout(ctx, capabilityTimeout)
	models, modes, err := loadAppServerCapabilities(appServerCtx, req)
	cancelAppServer()
	if err == nil {
		return buildCapabilities(models, modes), nil
	}

	// Older Codex builds may not expose app-server model/list. The debug catalog
	// is still structured JSON and preserves live model/effort/speed data.
	debugCtx, cancelDebug := context.WithTimeout(ctx, capabilityTimeout)
	defer cancelDebug()
	debugModels, debugErr := loadDebugModels(debugCtx, req)
	if debugErr == nil {
		caps := buildCapabilities(debugModels, collaborationModeListResponse{})
		caps.Warning = "Codex mode discovery is unavailable in this CLI version"
		return caps, nil
	}

	caps := fallbackCapabilities()
	caps.Warning = "Codex capabilities could not be read from the CLI"
	return caps, fmt.Errorf("codex capability discovery: %w", errors.Join(err, debugErr))
}

func loadAppServerCapabilities(
	ctx context.Context,
	req agent.CapabilityRequest,
) (modelListResponse, collaborationModeListResponse, error) {
	cmd := agent.NewCapabilityCommand(
		ctx,
		req,
		[]string{"HOME=/root", "CODEX_HOME=/root/.codex", "OPENAI_API_KEY="},
		"codex",
		"app-server",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return modelListResponse{}, collaborationModeListResponse{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return modelListResponse{}, collaborationModeListResponse{}, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return modelListResponse{}, collaborationModeListResponse{}, err
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	writeRPC := func(message any) error {
		return json.NewEncoder(stdin).Encode(message)
	}
	if err := writeRPC(map[string]any{
		"method": "initialize",
		"id":     1,
		"params": map[string]any{
			"clientInfo": map[string]string{
				"name":    "remote-futrx",
				"title":   "Remote",
				"version": "1",
			},
			"capabilities": map[string]bool{"experimentalApi": true},
		},
	}); err != nil {
		return modelListResponse{}, collaborationModeListResponse{}, err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	initialized := false
	var modelResult modelListResponse
	var modeResult collaborationModeListResponse
	modelsDone := false
	modesDone := false
	for scanner.Scan() {
		var response rpcResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil || response.ID == 0 {
			continue
		}
		switch response.ID {
		case 1:
			if response.Error != nil {
				return modelListResponse{}, collaborationModeListResponse{}, fmt.Errorf("initialize: %s", response.Error.Message)
			}
			if initialized {
				continue
			}
			initialized = true
			if err := writeRPC(map[string]any{
				"method": "model/list",
				"id":     2,
				"params": map[string]any{"includeHidden": false, "limit": 100},
			}); err != nil {
				return modelListResponse{}, collaborationModeListResponse{}, err
			}
			if err := writeRPC(map[string]any{
				"method": "collaborationMode/list",
				"id":     3,
				"params": map[string]any{},
			}); err != nil {
				return modelListResponse{}, collaborationModeListResponse{}, err
			}
		case 2:
			modelsDone = true
			if response.Error != nil {
				return modelListResponse{}, collaborationModeListResponse{}, fmt.Errorf("model/list: %s", response.Error.Message)
			}
			if err := json.Unmarshal(response.Result, &modelResult); err != nil {
				return modelListResponse{}, collaborationModeListResponse{}, fmt.Errorf("decode model/list: %w", err)
			}
		case 3:
			modesDone = true
			if response.Error == nil {
				_ = json.Unmarshal(response.Result, &modeResult)
			}
		}
		if modelsDone && modesDone {
			if len(modelResult.Data) == 0 {
				return modelListResponse{}, collaborationModeListResponse{}, errors.New("model/list returned no models")
			}
			return modelResult, modeResult, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return modelListResponse{}, collaborationModeListResponse{}, err
	}
	return modelListResponse{}, collaborationModeListResponse{}, errors.New("codex app-server closed before returning capabilities")
}

type debugCatalog struct {
	Models []debugModelItem `json:"models"`
}

type debugModelItem struct {
	Slug                  string               `json:"slug"`
	DisplayName           string               `json:"display_name"`
	Description           string               `json:"description"`
	DefaultReasoningLevel string               `json:"default_reasoning_level"`
	Visibility            string               `json:"visibility"`
	SupportedReasoning    []debugReasoningItem `json:"supported_reasoning_levels"`
	ServiceTiers          []serviceTierItem    `json:"service_tiers"`
}

type debugReasoningItem struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

func loadDebugModels(ctx context.Context, req agent.CapabilityRequest) (modelListResponse, error) {
	cmd := agent.NewCapabilityCommand(
		ctx,
		req,
		[]string{"HOME=/root", "CODEX_HOME=/root/.codex", "OPENAI_API_KEY="},
		"codex",
		"debug",
		"models",
	)
	output, err := cmd.Output()
	if err != nil {
		return modelListResponse{}, err
	}
	var debug debugCatalog
	if err := json.Unmarshal(output, &debug); err != nil {
		return modelListResponse{}, err
	}
	var result modelListResponse
	for _, model := range debug.Models {
		if model.Visibility != "" && model.Visibility != "list" {
			continue
		}
		item := modelListItem{
			ID: model.Slug, Model: model.Slug, DisplayName: model.DisplayName,
			Description: model.Description, DefaultReasoningEffort: model.DefaultReasoningLevel,
			IsDefault: len(result.Data) == 0,
		}
		for _, effort := range model.SupportedReasoning {
			item.SupportedReasoningEfforts = append(
				item.SupportedReasoningEfforts,
				reasoningEffortItem{ReasoningEffort: effort.Effort, Description: effort.Description},
			)
		}
		for _, tier := range model.ServiceTiers {
			item.ServiceTiers = append(item.ServiceTiers, serviceTierItem{
				ID: tier.ID, Name: tier.Name, Description: tier.Description,
			})
		}
		result.Data = append(result.Data, item)
	}
	if len(result.Data) == 0 {
		return modelListResponse{}, errors.New("codex debug catalog returned no visible models")
	}
	return result, nil
}

func buildCapabilities(models modelListResponse, providerModes collaborationModeListResponse) agent.Capabilities {
	items := make([]agent.ModelCapability, 0, len(models.Data))
	for _, raw := range models.Data {
		id := strings.TrimSpace(raw.Model)
		if id == "" {
			id = strings.TrimSpace(raw.ID)
		}
		if id == "" {
			continue
		}
		model := agent.ModelCapability{
			ID:                     id,
			Label:                  firstNonEmpty(raw.DisplayName, id),
			Description:            raw.Description,
			ProviderDefault:        raw.IsDefault,
			DefaultReasoningEffort: raw.DefaultReasoningEffort,
			DefaultServiceTier:     raw.DefaultServiceTier,
		}
		if len(raw.SupportedReasoningEfforts) > 0 {
			model.ReasoningEfforts = append(model.ReasoningEfforts, agent.AutoOption())
		}
		for _, effort := range raw.SupportedReasoningEfforts {
			model.ReasoningEfforts = append(model.ReasoningEfforts, agent.CapabilityOption{
				Value: effort.ReasoningEffort, Label: effortLabel(effort.ReasoningEffort), Description: effort.Description,
			})
		}
		if len(raw.ServiceTiers) > 0 {
			model.ServiceTiers = append(model.ServiceTiers, agent.AutoOption())
		}
		for _, tier := range raw.ServiceTiers {
			model.ServiceTiers = append(model.ServiceTiers, agent.CapabilityOption{
				Value: tier.ID, Label: firstNonEmpty(tier.Name, tier.ID), Description: tier.Description,
			})
		}
		items = append(items, model)
	}
	nativePlan := false
	for _, mode := range providerModes.Data {
		if strings.EqualFold(mode.Mode, "plan") {
			nativePlan = true
		}
	}
	return agent.Capabilities{
		Provider:    agent.ProviderCodex,
		Label:       "Codex",
		Source:      agent.CapabilitySourceLive,
		Models:      agent.WithAutoModel(items, "Codex default"),
		Modes:       agent.CodeAndPlanModes(nativePlan),
		DefaultMode: "code",
	}
}

func fallbackCapabilities() agent.Capabilities {
	return agent.Capabilities{
		Provider:    agent.ProviderCodex,
		Label:       "Codex",
		Source:      agent.CapabilitySourceFallback,
		Models:      agent.WithAutoModel(nil, "Codex default"),
		Modes:       agent.CodeAndPlanModes(false),
		DefaultMode: "code",
	}
}

func effortLabel(value string) string {
	if strings.EqualFold(value, "xhigh") {
		return "XHigh"
	}
	if value == "" {
		return "Auto"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
