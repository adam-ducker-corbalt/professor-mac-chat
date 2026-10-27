// Package chat answers ProfessorMAC questions with OpenAI: it runs the agent
// loop (chat.go), the tools the model can call (tools.go), and the sample
// data those tools read (sampledata.go). Register adds its /api routes.
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const openAIURL = "https://api.openai.com/v1/chat/completions"

// Sent ahead of every conversation. The data comes only from the tools in
// tools.go, so the model must not make any up.
const systemPrompt = `You are a customer support agent for ProfessorMAC, and you answer questions asked through ProfessorMAC in natural language. For example:
- How are DevOps metrics changing over the past three months?
- What are our greatest sources of cost savings?
- How has MACFin's deployment cadence changed since they shipped in February?

ProfessorMAC is a CMS DevSecOps observability tool with two modules:
- Savings Insights: cost-optimization findings across AWS (underutilized EC2, idle RDS, over-provisioned ECS, etc.)
- DevOps Center: DORA metrics (deployment frequency, lead time, change failure rate, recovery time) for CMCS teams.

Use the tools to look up data before answering, and base every number on tool results. Never invent metrics,
findings, team names, or figures. If the tools don't cover a question, say so. When comparing periods, give the
actual values for each period. The data is sample data, monthly from November 2025 through October 2026.

You can also answer weather questions for US locations with get_weather_forecast, which returns live data from the
National Weather Service. Say which place the forecast is for (the tool reports what the location matched) and that
it comes from the National Weather Service.

Answer in plain text (no Markdown headings or tables); short paragraphs and "-" bullet lists are fine.
Today is %s.`

const (
	maxMessages    = 50
	maxRequestBody = 64 << 10
	chatTimeout    = 120 * time.Second
	// Rounds of tool calls allowed per question, so a confused model can't loop forever.
	maxToolRounds = 6
)

// chatMessage is one turn of the conversation, as the browser sends it.
type chatMessage struct {
	Role string `json:"role"` // "user" or "assistant"
	Text string `json:"text"`
}

// toolUse records a tool the model called, for showing in the chat.
type toolUse struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON object, as the model wrote it
}

// apiMessage is a message in OpenAI's format. Besides system/user/assistant
// text, an assistant message can request tool calls, and a "tool" message
// carries one call's result back.
type apiMessage struct {
	Role       string     `json:"role"`
	Content    *string    `json:"content"` // null for an assistant message that only calls tools
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Client talks to OpenAI's Chat Completions API.
type Client struct {
	url       string // openAIURL, or a fake server in tests
	key       string
	model     string
	http      *http.Client
	logBodies bool // also log full request/response JSON (OPENAI_LOG_BODIES)
}

// NewClient returns a client for the given API key and model. With logBodies,
// every request and response is logged in full (never the key).
func NewClient(key, model string, logBodies bool) *Client {
	return &Client{url: openAIURL, key: key, model: model, http: &http.Client{}, logBodies: logBodies}
}

// Register adds the chat API to mux: POST /api/chat, the sample data the tools
// read at GET /api/sample/{teams,dora,findings}, and a JSON 404 for any other
// /api path. client is nil when no API key is configured.
func Register(mux *http.ServeMux, client *Client) {
	mux.HandleFunc("POST /api/chat", chatHandler(client))
	mux.HandleFunc("GET /api/sample/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, sampleTeams)
	})
	mux.HandleFunc("GET /api/sample/dora", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, doraMetrics())
	})
	mux.HandleFunc("GET /api/sample/findings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, sampleFindings)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
}

// answer runs the agent loop: send the conversation and tools to OpenAI; if
// the model asks for tools, run them, add the results, and ask again; once it
// replies with text, return that along with the tools it used.
func (c *Client) answer(ctx context.Context, conversation []chatMessage) (string, []toolUse, error) {
	prompt := fmt.Sprintf(systemPrompt, time.Now().Format("January 2, 2006"))
	messages := []apiMessage{{Role: "system", Content: &prompt}}
	for _, m := range conversation {
		messages = append(messages, apiMessage{Role: m.Role, Content: &m.Text})
	}

	var used []toolUse
	for range maxToolRounds {
		reply, err := c.complete(ctx, messages)
		if err != nil {
			return "", used, err
		}
		if len(reply.ToolCalls) == 0 {
			if reply.Content == nil {
				return "", used, errors.New("OpenAI returned an empty reply")
			}
			return *reply.Content, used, nil
		}

		// The model wants data. Keep its request in the conversation, then add
		// one "tool" message per call with the result.
		messages = append(messages, reply)
		for _, call := range reply.ToolCalls {
			result := runTool(ctx, call.Function.Name, call.Function.Arguments)
			log.Printf("tool %s%s -> %d bytes", call.Function.Name, call.Function.Arguments, len(result))
			messages = append(messages, apiMessage{Role: "tool", ToolCallID: call.ID, Content: &result})
			used = append(used, toolUse{Name: call.Function.Name, Arguments: call.Function.Arguments})
		}
	}
	return "", used, fmt.Errorf("no answer after %d rounds of tool calls", maxToolRounds)
}

// Numbers each OpenAI call in the log so requests and responses can be paired.
var openAICalls atomic.Int64

// complete makes one Chat Completions request and returns the model's message.
// Every call is logged: what was sent, how long it took, tokens used, and
// whether the model asked for tools or replied. With logBodies, the full
// request and response JSON are logged too (never the API key).
func (c *Client) complete(ctx context.Context, messages []apiMessage) (apiMessage, error) {
	body, err := json.Marshal(map[string]any{
		"model":    c.model,
		"messages": messages,
		"tools":    toolDefinitions,
	})
	if err != nil {
		return apiMessage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return apiMessage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	id := openAICalls.Add(1)
	log.Printf("openai #%d → %s: %d messages (%s), %d tools, latest: %s",
		id, c.model, len(messages), roleCounts(messages), len(toolDefinitions), describeMessage(messages[len(messages)-1]))
	if c.logBodies {
		log.Printf("openai #%d request body:\n%s", id, indentJSON(body))
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		log.Printf("openai #%d ✗ failed after %s: %v", id, time.Since(start).Round(time.Millisecond), err)
		return apiMessage{}, fmt.Errorf("calling OpenAI: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		log.Printf("openai #%d ✗ HTTP %d after %s, reading body: %v", id, resp.StatusCode, elapsed, err)
		return apiMessage{}, fmt.Errorf("reading OpenAI response: %w", err)
	}
	if c.logBodies {
		log.Printf("openai #%d response body:\n%s", id, indentJSON(raw))
	}

	var out struct {
		Choices []struct {
			Message apiMessage `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		log.Printf("openai #%d ✗ HTTP %d after %s, unreadable body", id, resp.StatusCode, elapsed)
		return apiMessage{}, fmt.Errorf("OpenAI returned HTTP %d with an unreadable body", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		message := fmt.Sprintf("OpenAI returned HTTP %d", resp.StatusCode)
		if out.Error != nil {
			message = "OpenAI: " + out.Error.Message
		}
		log.Printf("openai #%d ✗ HTTP %d after %s: %s", id, resp.StatusCode, elapsed, message)
		return apiMessage{}, errors.New(message)
	}
	if len(out.Choices) == 0 {
		log.Printf("openai #%d ✗ no choices after %s", id, elapsed)
		return apiMessage{}, errors.New("OpenAI returned no reply")
	}

	reply := out.Choices[0].Message
	log.Printf("openai #%d ← %s, %d tokens in / %d out: %s",
		id, elapsed, out.Usage.PromptTokens, out.Usage.CompletionTokens, describeMessage(reply))
	return reply, nil
}

// roleCounts summarizes a conversation, e.g. "system 1, user 1, assistant 1, tool 2".
func roleCounts(messages []apiMessage) string {
	counts := map[string]int{}
	for _, m := range messages {
		counts[m.Role]++
	}
	var parts []string
	for _, role := range []string{"system", "user", "assistant", "tool"} {
		if counts[role] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", role, counts[role]))
		}
	}
	return strings.Join(parts, ", ")
}

// describeMessage is a one-line summary of a message for the log.
func describeMessage(m apiMessage) string {
	if len(m.ToolCalls) > 0 {
		var calls []string
		for _, call := range m.ToolCalls {
			calls = append(calls, call.Function.Name+call.Function.Arguments)
		}
		return "asks for tools " + strings.Join(calls, ", ")
	}
	text := ""
	if m.Content != nil {
		text = *m.Content
	}
	if m.Role == "tool" {
		return fmt.Sprintf("tool result (%d bytes)", len(text))
	}
	return fmt.Sprintf("%s %q", m.Role, preview(text, 120))
}

// preview shortens text to one line of at most n characters.
func preview(text string, n int) string {
	text = strings.Join(strings.Fields(text), " ")
	if r := []rune(text); len(r) > n {
		return string(r[:n]) + "…"
	}
	return text
}

func indentJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}

// chatHandler answers POST /api/chat with {"messages": [...]} by returning
// {"reply": "...", "tools": [...]}, or {"error": "..."} on failure. client is
// nil when no API key is configured.
func chatHandler(client *Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "OPENAI_API_KEY is not set on the server; add it to .env and restart",
			})
			return
		}

		var req struct {
			Messages []chatMessage `json:"messages"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := validateConversation(req.Messages); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), chatTimeout)
		defer cancel()
		start := time.Now()
		reply, used, err := client.answer(ctx, req.Messages)
		if err != nil {
			log.Printf("chat: %d messages, failed after %s: %v", len(req.Messages), time.Since(start).Round(time.Millisecond), err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		log.Printf("chat: %d messages, %d tool calls, answered in %s", len(req.Messages), len(used), time.Since(start).Round(time.Millisecond))
		if used == nil {
			used = []toolUse{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"reply": reply, "tools": used})
	}
}

func validateConversation(messages []chatMessage) error {
	if len(messages) == 0 {
		return errors.New("no messages")
	}
	if len(messages) > maxMessages {
		return fmt.Errorf("conversation is too long (max %d messages)", maxMessages)
	}
	for _, m := range messages {
		if m.Role != "user" && m.Role != "assistant" {
			return fmt.Errorf("invalid role %q", m.Role)
		}
	}
	if messages[len(messages)-1].Role != "user" {
		return errors.New("the last message must be from the user")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
