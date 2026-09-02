package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	configconstants "github.com/futrx-com/remote.futrx.com/internal/config/constants"
	agentauth "github.com/futrx-com/remote.futrx.com/internal/service/agent/auth"
)

const maxAPIKeyValidationResponseBytes = 1 << 20

var ErrAPIKeyValidationUnavailable = errors.New("MiniMax API key validation is temporarily unavailable")

type apiKeyValidationClient interface {
	Do(*http.Request) (*http.Response, error)
}

type apiKeyValidator struct {
	client   apiKeyValidationClient
	endpoint string
}

func newAPIKeyValidator() *apiKeyValidator {
	return &apiKeyValidator{
		client: &http.Client{
			Timeout: configconstants.MiniMaxAPIValidationTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		endpoint: configconstants.MiniMaxAPIBaseURL + "/models",
	}
}

func (v *apiKeyValidator) ValidateAPIKey(ctx context.Context, key string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.endpoint, nil)
	if err != nil {
		return ErrAPIKeyValidationUnavailable
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+key)

	response, err := v.client.Do(request)
	if err != nil {
		return ErrAPIKeyValidationUnavailable
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return agentauth.ErrAPIKeyRejected
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w (HTTP %d)", ErrAPIKeyValidationUnavailable, response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxAPIKeyValidationResponseBytes+1))
	if err != nil || len(body) > maxAPIKeyValidationResponseBytes {
		return ErrAPIKeyValidationUnavailable
	}
	var models struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &models); err != nil || models.Object != "list" || models.Data == nil {
		return ErrAPIKeyValidationUnavailable
	}
	return nil
}

var _ agentauth.APIKeyValidator = (*apiKeyValidator)(nil)
