package codex

import (
	"context"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

type appServerRequestHandler struct {
	mode      agent.RunMode
	write     func(any) error
	userInput *appServerUserInputHandler
}

func newAppServerRequestHandler(
	req agent.RunRequest,
	write func(any) error,
) *appServerRequestHandler {
	return &appServerRequestHandler{
		mode:      req.Mode,
		write:     write,
		userInput: newAppServerUserInputHandler(req.Interact),
	}
}

func (handler *appServerRequestHandler) Answer(ctx context.Context, envelope appServerEnvelope) error {
	return handler.answer(ctx, envelope, nil)
}

func (handler *appServerRequestHandler) answer(
	ctx context.Context,
	envelope appServerEnvelope,
	registered func(),
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := any(nil)
	switch envelope.Method {
	case "item/tool/requestUserInput", "tool/requestUserInput":
		userInputResult, err := handler.userInput.answer(ctx, envelope, registered)
		if err != nil {
			return err
		}
		result = userInputResult

	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		decision := "accept"
		if handler.mode == agent.RunModePlan {
			decision = "decline"
		}
		result = map[string]string{"decision": decision}

	case "execCommandApproval", "applyPatchApproval":
		if handler.mode == agent.RunModePlan {
			result = map[string]any{"decision": map[string]any{
				"denied": map[string]string{"rejection": "Plan mode does not allow mutations"},
			}}
		} else {
			result = map[string]string{"decision": "approved"}
		}

	case "mcpServer/elicitation/request":
		result = map[string]any{"action": "cancel", "content": nil}

	default:
		if err := ctx.Err(); err != nil {
			return err
		}
		return handler.write(map[string]any{
			"id": envelope.ID,
			"error": map[string]any{
				"code":    -32601,
				"message": "Remote does not implement " + envelope.Method,
			},
		})
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return handler.write(map[string]any{"id": envelope.ID, "result": result})
}
