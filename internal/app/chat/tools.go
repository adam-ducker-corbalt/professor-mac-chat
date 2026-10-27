package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// The tools the model can call. Each has a name, a description the model reads
// to decide when to use it, and a JSON Schema for its arguments. These are sent
// to OpenAI with every request; Go runs the tool when the model asks for it.
var toolDefinitions = []map[string]any{
	{
		"type": "function",
		"function": map[string]any{
			"name":        "list_teams",
			"description": "List the teams in ProfessorMAC with a short description of each. Use it to find a team's exact name.",
			"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
		},
	},
	{
		"type": "function",
		"function": map[string]any{
			"name": "get_dora_metrics",
			"description": "Monthly DORA metrics per team: deployments (count), lead_time_hours (median commit to production), " +
				"change_failure_rate (percent of deployments causing a failure) and recovery_hours (median time to restore). " +
				"Data covers 2025-11 through 2026-10.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"team":        map[string]any{"type": "string", "description": "Team name, e.g. MACFin. Omit for all teams."},
					"start_month": map[string]any{"type": "string", "description": "First month to include, YYYY-MM. Omit for the earliest."},
					"end_month":   map[string]any{"type": "string", "description": "Last month to include, YYYY-MM. Omit for the latest."},
				},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]any{
			"name": "get_cost_findings",
			"description": "Cost-savings findings from Savings Insights, with estimated monthly savings in USD, plus totals " +
				"by AWS service, finding type and team. Status is open (not yet fixed), accepted (team chose not to fix) " +
				"or remediated (fixed, savings realized).",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "enum": []string{"open", "accepted", "remediated"}, "description": "Only findings with this status. Omit for all."},
					"team":   map[string]any{"type": "string", "description": "Only findings for this team. Omit for all teams."},
				},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]any{
			"name": "get_weather_forecast",
			"description": "Live weather from the National Weather Service for a US location: current conditions at the nearest " +
				"station and the forecast for the next four half-day periods (e.g. Today, Tonight, Wednesday, Wednesday Night). " +
				"US locations only.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{"type": "string", "description": "Where to get weather for: a city, \"city, state\", ZIP code, address, or landmark, e.g. \"Joplin, MO\" or \"90210\"."},
				},
				"required": []string{"location"},
			},
		},
	},
}

// runTool runs one tool call and returns its result as JSON for the model.
// Problems (bad arguments, unknown team, unknown place) are returned as an
// "error" field so the model can correct itself instead of the request failing.
func runTool(ctx context.Context, name, arguments string) string {
	var args map[string]string
	if strings.TrimSpace(arguments) != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return toolError("arguments must be a JSON object of strings")
		}
	}

	var result any
	var err error
	switch name {
	case "list_teams":
		result = sampleTeams
	case "get_dora_metrics":
		result, err = getDoraMetrics(args["team"], args["start_month"], args["end_month"])
	case "get_cost_findings":
		result, err = getCostFindings(args["status"], args["team"])
	case "get_weather_forecast":
		result, err = getWeatherForecast(ctx, args["location"])
	default:
		err = fmt.Errorf("unknown tool %q", name)
	}
	if err != nil {
		return toolError(err.Error())
	}

	out, err := json.Marshal(result)
	if err != nil {
		return toolError(err.Error())
	}
	return string(out)
}

func toolError(message string) string {
	out, _ := json.Marshal(map[string]string{"error": message})
	return string(out)
}

// teamName matches a team case-insensitively ("macfin" -> "MACFin").
func teamName(name string) (string, error) {
	var names []string
	for _, t := range sampleTeams {
		if strings.EqualFold(t.Name, name) {
			return t.Name, nil
		}
		names = append(names, t.Name)
	}
	return "", fmt.Errorf("unknown team %q; teams are %s", name, strings.Join(names, ", "))
}

func getDoraMetrics(team, start, end string) ([]DoraMonth, error) {
	if team != "" {
		var err error
		if team, err = teamName(team); err != nil {
			return nil, err
		}
	}
	for _, m := range []string{start, end} {
		if m != "" && !slices.Contains(sampleMonths, m) {
			return nil, fmt.Errorf("month %q is outside the data, which covers %s through %s", m, sampleMonths[0], sampleMonths[len(sampleMonths)-1])
		}
	}

	var rows []DoraMonth
	for _, r := range doraMetrics() {
		// YYYY-MM strings sort chronologically.
		if (team == "" || r.Team == team) && (start == "" || r.Month >= start) && (end == "" || r.Month <= end) {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

type findingsResult struct {
	Findings            []Finding          `json:"findings"`
	TotalMonthlySavings float64            `json:"total_monthly_savings_usd"`
	ByService           map[string]float64 `json:"monthly_savings_by_service"`
	ByType              map[string]float64 `json:"monthly_savings_by_type"`
	ByTeam              map[string]float64 `json:"monthly_savings_by_team"`
}

func getCostFindings(status, team string) (findingsResult, error) {
	if team != "" {
		var err error
		if team, err = teamName(team); err != nil {
			return findingsResult{}, err
		}
	}
	if status != "" && status != "open" && status != "accepted" && status != "remediated" {
		return findingsResult{}, fmt.Errorf("status must be open, accepted or remediated")
	}

	result := findingsResult{
		Findings:  []Finding{},
		ByService: map[string]float64{},
		ByType:    map[string]float64{},
		ByTeam:    map[string]float64{},
	}
	for _, f := range sampleFindings {
		if (status != "" && f.Status != status) || (team != "" && f.Team != team) {
			continue
		}
		result.Findings = append(result.Findings, f)
		result.TotalMonthlySavings += f.MonthlySavings
		result.ByService[f.Service] += f.MonthlySavings
		result.ByType[f.Service+": "+f.Type] += f.MonthlySavings
		result.ByTeam[f.Team] += f.MonthlySavings
	}
	return result, nil
}
