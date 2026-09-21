package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type accountReadResponse struct {
	Account *struct {
		Type     string `json:"type"`
		Email    string `json:"email"`
		PlanType string `json:"planType"`
	} `json:"account"`
	RequiresOpenAIAuth bool `json:"requiresOpenaiAuth"`
}

func validateAccountCredential(ctx context.Context, credential []byte) (validatedAccount, error) {
	if len(credential) == 0 {
		return validatedAccount{}, errors.New("saved Codex credential is empty")
	}
	var raw map[string]any
	if err := json.Unmarshal(credential, &raw); err != nil {
		return validatedAccount{}, errors.New("saved Codex credential is not valid JSON")
	}
	mode, usesAPIKey := codexAuthModeFromRaw(raw)
	if usesAPIKey || mode != "chatgpt" {
		return validatedAccount{}, errors.New("saved Codex account is not a ChatGPT subscription login")
	}

	home, err := os.MkdirTemp("", "remote-codex-validate-*")
	if err != nil {
		return validatedAccount{}, fmt.Errorf("prepare Codex validation: %w", err)
	}
	defer os.RemoveAll(home)
	if err := os.Chmod(home, 0o700); err != nil {
		return validatedAccount{}, err
	}
	if err := os.WriteFile(filepath.Join(home, "auth.json"), credential, 0o600); err != nil {
		return validatedAccount{}, err
	}

	account, err := readCodexAccount(ctx, home)
	if err != nil {
		return validatedAccount{}, fmt.Errorf("validate Codex account: %w", err)
	}
	if account.Account == nil || account.Account.Type != "chatgpt" {
		return validatedAccount{}, errors.New("Codex did not recognize a ChatGPT account")
	}
	refreshed, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		return validatedAccount{}, fmt.Errorf("read refreshed Codex credential: %w", err)
	}
	return validatedAccount{
		Email: account.Account.Email, PlanType: account.Account.PlanType,
		Credential: append(json.RawMessage(nil), refreshed...),
	}, nil
}

func codexAuthModeFromRaw(raw map[string]any) (string, bool) {
	mode, _ := raw["auth_mode"].(string)
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		_, usesAPIKey := raw["OPENAI_API_KEY"]
		return mode, usesAPIKey
	}
	return mode, mode == "apikey"
}

func readCodexAccount(ctx context.Context, home string) (accountReadResponse, error) {
	cmd := exec.CommandContext(ctx, "codex", "app-server")
	cmd.Env = codexAuthEnvFor(os.Environ(), home)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return accountReadResponse{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return accountReadResponse{}, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return accountReadResponse{}, err
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	return readCodexAccountRPC(stdin, stdout)
}

func readCodexAccountRPC(stdin io.Writer, stdout io.Reader) (accountReadResponse, error) {
	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(map[string]any{
		"method": "initialize", "id": 1,
		"params": map[string]any{
			"clientInfo":   map[string]string{"name": "remote-futrx", "title": "Remote", "version": "1"},
			"capabilities": map[string]bool{"experimentalApi": true},
		},
	}); err != nil {
		return accountReadResponse{}, err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var response rpcResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil || response.ID == 0 {
			continue
		}
		switch response.ID {
		case 1:
			if response.Error != nil {
				return accountReadResponse{}, fmt.Errorf("initialize: %s", response.Error.Message)
			}
			if err := encoder.Encode(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
				return accountReadResponse{}, err
			}
			if err := encoder.Encode(map[string]any{
				"method": "account/read", "id": 2,
				"params": map[string]bool{"refreshToken": true},
			}); err != nil {
				return accountReadResponse{}, err
			}
		case 2:
			if response.Error != nil {
				return accountReadResponse{}, errors.New(response.Error.Message)
			}
			var account accountReadResponse
			if err := json.Unmarshal(response.Result, &account); err != nil {
				return accountReadResponse{}, fmt.Errorf("decode account/read: %w", err)
			}
			return account, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return accountReadResponse{}, err
	}
	return accountReadResponse{}, errors.New("Codex app-server closed before returning account status")
}
