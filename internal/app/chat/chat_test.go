package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A fake OpenAI that asks for MACFin's DORA metrics on the first request, then
// answers once it sees the tool result. Checks the whole loop without an API key.
func TestAnswerRunsToolsThenReplies(t *testing.T) {
	var requests []map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		requests = append(requests, body)

		if len(requests) == 1 {
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_1","type":"function","function":{"name":"get_dora_metrics","arguments":"{\"team\":\"macfin\",\"start_month\":\"2026-02\"}"}}
			]}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"MACFin went from 2 to 17 deployments a month."}}]}`))
	}))
	defer fake.Close()

	client := &Client{url: fake.URL, key: "test", model: "test-model", http: fake.Client()}
	reply, used, err := client.answer(context.Background(), []chatMessage{{Role: "user", Text: "How has MACFin changed?"}})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "MACFin went from 2 to 17 deployments a month." {
		t.Errorf("reply = %q", reply)
	}
	if len(used) != 1 || used[0].Name != "get_dora_metrics" {
		t.Errorf("used = %+v", used)
	}
	if len(requests) != 2 {
		t.Fatalf("made %d requests to OpenAI, want 2", len(requests))
	}

	// Every request offers the tools.
	if tools, _ := requests[0]["tools"].([]any); len(tools) != len(toolDefinitions) {
		t.Errorf("first request sent %d tools, want %d", len(tools), len(toolDefinitions))
	}

	// The second request carries the model's tool call and our result:
	// system, user, assistant (tool_calls), tool.
	messages := requests[1]["messages"].([]any)
	if len(messages) != 4 {
		t.Fatalf("second request has %d messages, want 4", len(messages))
	}
	toolMsg := messages[3].(map[string]any)
	if toolMsg["role"] != "tool" || toolMsg["tool_call_id"] != "call_1" {
		t.Errorf("tool message = %v", toolMsg)
	}
	var rows []DoraMonth
	if err := json.Unmarshal([]byte(toolMsg["content"].(string)), &rows); err != nil {
		t.Fatalf("tool result isn't DORA rows: %v", err)
	}
	if len(rows) != 9 || rows[0].Month != "2026-02" || rows[0].Team != "MACFin" {
		t.Errorf("got %d rows starting %+v; want 9 MACFin rows from 2026-02", len(rows), rows[0])
	}
}

func TestRunToolErrorsAreReturnedToTheModel(t *testing.T) {
	for _, tc := range []struct{ name, args, want string }{
		{"get_dora_metrics", `{"team":"Nope"}`, "unknown team"},
		{"get_dora_metrics", `{"start_month":"2024-01"}`, "outside the data"},
		{"get_cost_findings", `{"status":"closed"}`, "status must be"},
		{"get_weather_forecast", `{}`, "location is required"},
		{"delete_everything", `{}`, "unknown tool"},
	} {
		if got := runTool(context.Background(), tc.name, tc.args); !strings.Contains(got, tc.want) {
			t.Errorf("runTool(%s, %s) = %s; want error containing %q", tc.name, tc.args, got, tc.want)
		}
	}
}

func TestCostFindingsTotals(t *testing.T) {
	result, err := getCostFindings("open", "")
	if err != nil {
		t.Fatal(err)
	}
	// EC2 should be the biggest open category, as planted in sampledata.go.
	for service, total := range result.ByService {
		if service != "EC2" && total >= result.ByService["EC2"] {
			t.Errorf("%s ($%.0f) >= EC2 ($%.0f); EC2 should be biggest", service, total, result.ByService["EC2"])
		}
	}
}
