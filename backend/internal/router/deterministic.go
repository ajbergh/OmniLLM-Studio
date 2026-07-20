package router

import (
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sports"
)

func deterministicSportsRoute(message string, now time.Time) (RouterDecision, bool) {
	req, ok := sports.DetectSportsIntent(message, now)
	if !ok || req == nil {
		return RouterDecision{}, false
	}
	params := &SportsRouteParams{
		Intent:             string(req.Intent),
		League:             req.League,
		Sport:              req.Sport,
		TeamQuery:          req.TeamQuery,
		AthleteQuery:       req.AthleteQuery,
		SecondAthleteQuery: req.SecondAthleteQuery,
		Metric:             req.StatName,
		DateLabel:          req.DateLabel,
		GameDetailSubtype:  req.GameDetailSubtype,
	}
	if req.Date != nil {
		params.Date = req.Date.Format("2006-01-02")
	}
	if req.Season > 0 {
		season := req.Season
		params.Season = &season
	}
	if req.Limit > 0 {
		limit := req.Limit
		params.Limit = &limit
	}
	return RouterDecision{
		Route:                 RouteSportsLookup,
		Confidence:            1,
		RequiresGenerationLLM: false,
		Reason:                "deterministic sports intent",
		Sports:                params,
	}, true
}
