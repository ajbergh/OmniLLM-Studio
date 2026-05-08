package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/tools"
)

type SportsLookupTool struct {
	client *ESPNClient
	now    func() time.Time
}

func NewSportsLookupTool(client *ESPNClient) *SportsLookupTool {
	if client == nil {
		client = NewESPNClient()
	}
	return &SportsLookupTool{
		client: client,
		now:    time.Now,
	}
}

type sportsLookupArgs struct {
	Query  string `json:"query"`
	Intent string `json:"intent,omitempty"`
	League string `json:"league,omitempty"`
	Date   string `json:"date,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type sportsLookupOutput struct {
	Intent      SportsIntentType `json:"intent"`
	League      string           `json:"league"`
	LeagueName  string           `json:"league_name"`
	Markdown    string           `json:"markdown"`
	Source      string           `json:"source"`
	RetrievedAt string           `json:"retrieved_at"`
}

func (t *SportsLookupTool) Definition() tools.ToolDefinition {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Raw user request, e.g. 'Show me today's NBA scores'"
			},
			"intent": {
				"type": "string",
				"enum": ["standings", "scores", "schedule", "news", "roster", "injuries", "transactions", "team_record", "team_schedule", "leaders", "athlete_stats", "athlete_news", "rankings", "league_stats"],
				"description": "Optional explicit sports intent"
			},
			"league": {
				"type": "string",
				"description": "Optional league alias like mlb, nfl, nba, nhl, epl, mls"
			},
			"date": {
				"type": "string",
				"description": "Optional date in YYYY-MM-DD, or today, tomorrow, yesterday"
			},
			"limit": {
				"type": "integer",
				"description": "Optional maximum number of rows or articles to return"
			}
		},
		"required": ["query"]
	}`)

	return tools.ToolDefinition{
		Name:        "sports_lookup",
		Description: "Fetch ESPN-backed sports scores, schedules, standings, news, rosters, injuries, transactions, team records, rankings, player stats, league stats, and league leaders. Use this for current or ESPN-specific sports questions instead of answering from model memory.",
		Parameters:  schema,
		Category:    "sports",
		Enabled:     true,
	}
}

func (t *SportsLookupTool) Validate(args json.RawMessage) error {
	var a sportsLookupArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if strings.TrimSpace(a.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if a.Intent != "" {
		switch SportsIntentType(strings.ToLower(strings.TrimSpace(a.Intent))) {
		case SportsIntentStandings, SportsIntentScores, SportsIntentSchedule, SportsIntentNews,
			SportsIntentRoster, SportsIntentInjuries, SportsIntentTransactions,
			SportsIntentTeamRecord, SportsIntentTeamSchedule, SportsIntentLeaders,
			SportsIntentAthleteStats, SportsIntentAthleteNews, SportsIntentRankings, SportsIntentLeagueStats:
		default:
			return fmt.Errorf("unsupported sports intent")
		}
	}
	return nil
}

func (t *SportsLookupTool) Execute(ctx context.Context, args json.RawMessage) (*tools.ToolResult, error) {
	var a sportsLookupArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	now := t.timeNow()
	req, ok := DetectSportsIntent(a.Query, now)
	if !ok {
		req = &SportsRequest{
			RawQuery: strings.TrimSpace(a.Query),
			Limit:    defaultLimitForIntent(SportsIntentUnknown),
		}
	} else if err := ValidateDateInQuery(a.Query, now); err != nil {
		return &tools.ToolResult{
			Content: UserFacingError(*req, err),
			IsError: true,
		}, nil
	}

	if a.Intent != "" {
		req.Intent = SportsIntentType(strings.ToLower(strings.TrimSpace(a.Intent)))
		if req.Limit == defaultLimitForIntent(SportsIntentUnknown) {
			req.Limit = defaultLimitForIntent(req.Intent)
		}
	}
	if a.League != "" {
		cfg, ok := leagueConfigByAlias(a.League)
		if !ok {
			cfg, ok = leagueConfigByLeague(a.League)
		}
		if !ok {
			return &tools.ToolResult{
				Content: UserFacingError(*req, ErrUnsupportedLeague),
				IsError: true,
			}, nil
		}
		req.League = cfg.League
		req.Sport = cfg.Sport
	}
	if a.Date != "" {
		date, label, err := ParseDateValue(a.Date, now, req.Intent)
		if err != nil {
			return &tools.ToolResult{
				Content: UserFacingError(*req, err),
				IsError: true,
			}, nil
		}
		req.Date = date
		req.DateLabel = label
	}
	if a.Limit > 0 {
		req.Limit = a.Limit
	}

	result, err := t.client.Lookup(ctx, *req)
	if err != nil {
		return &tools.ToolResult{
			Content: UserFacingError(*req, err),
			IsError: true,
			Metadata: map[string]interface{}{
				"tool":   "sports_lookup",
				"source": SourceESPN,
				"error":  err.Error(),
			},
		}, nil
	}

	out := sportsLookupOutput{
		Intent:      result.Intent,
		League:      result.League,
		LeagueName:  result.LeagueName,
		Markdown:    result.Markdown,
		Source:      result.Source,
		RetrievedAt: result.RetrievedAt.Format(time.RFC3339),
	}
	content, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal sports result: %w", err)
	}

	return &tools.ToolResult{
		Content: string(content),
		Metadata: map[string]interface{}{
			"tool":         "sports_lookup",
			"intent":       result.Intent,
			"league":       result.League,
			"league_name":  result.LeagueName,
			"source":       result.Source,
			"retrieved_at": result.RetrievedAt.Format(time.RFC3339),
			"markdown":     result.Markdown,
		},
	}, nil
}

func (t *SportsLookupTool) timeNow() time.Time {
	if t != nil && t.now != nil {
		return t.now()
	}
	return time.Now()
}
