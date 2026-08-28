package codex

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

const nonBlockingUserInputAutoResolutionMS int64 = 120_000

type appServerUserInputHandler struct {
	interact agent.InteractionHandler
}

func newAppServerUserInputHandler(interact agent.InteractionHandler) *appServerUserInputHandler {
	return &appServerUserInputHandler{interact: interact}
}

// answer adapts Codex's native requestUserInput request and response
// shapes to Remote's provider-neutral interaction contract. The generic
// app-server request handler remains responsible only for protocol dispatch
// and writing the resulting JSON-RPC response.
func (handler *appServerUserInputHandler) answer(
	ctx context.Context,
	envelope appServerEnvelope,
	registered func(),
) (any, error) {
	var params appServerUserInputRequestParams
	if err := json.Unmarshal(envelope.Params, &params); err != nil {
		return nil, err
	}
	if len(params.Questions) == 0 {
		return nil, errors.New("Codex user-input request contains no questions")
	}
	questionsByID := make(map[string]appServerUserQuestion, len(params.Questions))
	for _, question := range params.Questions {
		questionID := strings.TrimSpace(question.ID)
		if questionID == "" {
			return nil, errors.New("Codex user-input request contains a question without an id")
		}
		if _, duplicate := questionsByID[questionID]; duplicate {
			return nil, errors.New("Codex user-input request contains duplicate question ids")
		}
		questionsByID[questionID] = question
	}
	inputData := map[string]any{"questions": params.Questions}
	if params.IsBlocking != nil {
		inputData["isBlocking"] = *params.IsBlocking
	}
	if params.AutoResolutionMS != nil {
		inputData["autoResolutionMs"] = *params.AutoResolutionMS
	}
	input, _ := json.Marshal(inputData)
	if handler.interact == nil {
		return nil, errors.New("Codex requested user input but no interaction handler is available")
	}
	interactionID := strings.TrimSpace(params.ItemID)
	if interactionID == "" {
		interactionID = strings.Trim(strings.TrimSpace(string(envelope.ID)), `"`)
	}
	blocking := true
	if params.IsBlocking != nil {
		blocking = *params.IsBlocking
	}
	autoResolutionMS := int64(0)
	if !blocking {
		autoResolutionMS = nonBlockingUserInputAutoResolutionMS
	}
	sensitive := false
	for _, question := range params.Questions {
		sensitive = sensitive || question.IsSecret
	}
	response, err := handler.interact(ctx, agent.InteractionRequest{
		ID:               interactionID,
		Kind:             agent.InteractionUserInput,
		ToolName:         "AskUserQuestion",
		Input:            input,
		Blocking:         blocking,
		Sensitive:        sensitive,
		AutoResolutionMS: autoResolutionMS,
		Registered:       registered,
	})
	if err != nil {
		return nil, err
	}
	answers := make(map[string]any, len(response.Answers))
	for questionID, selected := range response.Answers {
		question, exists := questionsByID[questionID]
		if !exists {
			continue
		}
		if selected == nil {
			selected = []string{}
		}
		answers[questionID] = map[string]any{
			"answers": codexUserInputAnswers(question, selected),
		}
	}
	return map[string]any{"answers": answers}, nil
}

func codexUserInputAnswers(question appServerUserQuestion, selected []string) []string {
	optionLabels := make(map[string]struct{}, len(question.Options))
	for _, option := range question.Options {
		optionLabels[option.Label] = struct{}{}
	}
	answers := make([]string, 0, len(selected))
	for _, answer := range selected {
		if _, isOption := optionLabels[answer]; isOption {
			answers = append(answers, answer)
			continue
		}
		if strings.HasPrefix(answer, "user_note: ") {
			answers = append(answers, answer)
			continue
		}
		answers = append(answers, "user_note: "+answer)
	}
	return answers
}
