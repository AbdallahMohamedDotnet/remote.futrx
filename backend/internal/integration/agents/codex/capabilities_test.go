package codex

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/futrx-com/remote.futrx.com/internal/agent"
)

func TestReadAppServerCapabilitiesInitializesBeforePaginatingModelCatalog(t *testing.T) {
	serverRequestReader, clientRequestWriter := io.Pipe()
	clientResponseReader, serverResponseWriter := io.Pipe()
	serverDone := make(chan error, 1)

	type request struct {
		ID     int            `json:"id"`
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}
	go func() {
		defer serverRequestReader.Close()
		defer serverResponseWriter.Close()

		decoder := json.NewDecoder(serverRequestReader)
		encoder := json.NewEncoder(serverResponseWriter)
		readRequest := func(wantMethod string, wantID int) (request, error) {
			var got request
			if err := decoder.Decode(&got); err != nil {
				return request{}, err
			}
			if got.Method != wantMethod || got.ID != wantID {
				return request{}, fmt.Errorf("request = %+v, want method %q id %d", got, wantMethod, wantID)
			}
			return got, nil
		}

		if _, err := readRequest("initialize", 1); err != nil {
			serverDone <- err
			return
		}
		if err := encoder.Encode(map[string]any{"id": 1, "result": map[string]any{}}); err != nil {
			serverDone <- err
			return
		}
		if _, err := readRequest("initialized", 0); err != nil {
			serverDone <- err
			return
		}
		firstPage, err := readRequest("model/list", 2)
		if err != nil {
			serverDone <- err
			return
		}
		if _, exists := firstPage.Params["cursor"]; exists {
			serverDone <- fmt.Errorf("first model request has cursor: %+v", firstPage)
			return
		}
		if _, err := readRequest("collaborationMode/list", 3); err != nil {
			serverDone <- err
			return
		}
		if err := encoder.Encode(map[string]any{
			"id":     3,
			"result": map[string]any{"data": []map[string]string{{"name": "Plan", "mode": "plan"}}},
		}); err != nil {
			serverDone <- err
			return
		}
		if err := encoder.Encode(map[string]any{
			"id": 2,
			"result": map[string]any{
				"data":       []map[string]string{{"id": "first", "model": "first", "displayName": "First"}},
				"nextCursor": "page-2",
			},
		}); err != nil {
			serverDone <- err
			return
		}
		secondPage, err := readRequest("model/list", 4)
		if err != nil {
			serverDone <- err
			return
		}
		if secondPage.Params["cursor"] != "page-2" {
			serverDone <- fmt.Errorf("second model request = %+v", secondPage)
			return
		}
		if err := encoder.Encode(map[string]any{
			"id": 4,
			"result": map[string]any{
				"data": []map[string]string{{"id": "second", "model": "second", "displayName": "Second"}},
			},
		}); err != nil {
			serverDone <- err
			return
		}
		serverDone <- nil
	}()

	models, modes, err := readAppServerCapabilities(clientRequestWriter, clientResponseReader)
	_ = clientRequestWriter.Close()
	_ = clientResponseReader.Close()
	if serverErr := <-serverDone; serverErr != nil {
		t.Fatal(serverErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(models.Data) != 2 || models.Data[0].ID != "first" || models.Data[1].ID != "second" {
		t.Fatalf("models = %+v", models.Data)
	}
	if len(modes.Data) != 1 || modes.Data[0].Mode != string(agent.RunModePlan) {
		t.Fatalf("modes = %+v", modes.Data)
	}
}

func TestBuildCapabilitiesPreservesPerModelControlsWithoutAdvertisingPlan(t *testing.T) {
	var models modelListResponse
	models.Data = append(models.Data, modelListItem{
		ID: "gpt-next", Model: "gpt-next", DisplayName: "GPT Next", IsDefault: true,
		DefaultReasoningEffort: "medium",
		SupportedReasoningEfforts: []reasoningEffortItem{{
			ReasoningEffort: "medium", Description: "balanced",
		}},
		ServiceTiers: []serviceTierItem{{ID: "priority", Name: "Fast", Description: "faster"}},
	})
	modes := collaborationModeListResponse{}
	modes.Data = append(modes.Data, collaborationModeItem{Name: "Plan", Mode: string(agent.RunModePlan)})

	caps := buildCapabilities(models, modes)
	if len(caps.Models) != 2 || caps.Models[0].ID != "" || caps.Models[1].ID != "gpt-next" {
		t.Fatalf("models = %+v", caps.Models)
	}
	if got := caps.Models[1].ReasoningEfforts; len(got) != 2 || got[1].Value != "medium" {
		t.Fatalf("reasoning efforts = %+v", got)
	}
	if got := caps.Models[1].ServiceTiers; len(got) != 2 || got[1].Value != "priority" || got[1].Label != "Fast" {
		t.Fatalf("service tiers = %+v", got)
	}
	if len(caps.Modes) != 1 || caps.Modes[0].Value != string(agent.RunModeDefault) {
		t.Fatalf("modes = %+v", caps.Modes)
	}
}
