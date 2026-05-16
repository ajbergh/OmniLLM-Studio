package sports

// advanced_extra.go — Lookup methods for Q77–Q87 (Venues, Power Index,
// Recruits, Bracketology) and shared helpers used by those methods.

import (
	"context"
	"fmt"
	"strings"

	espn "github.com/chinmaykhachane/espn-go"
)

// ─── Q77–Q78: Venues / Stadiums ──────────────────────────────────────────────

// LookupVenues returns the list of venue refs for a league. Because the ESPN
// core Venues endpoint returns only $ref links (not full venue documents),
// the result is a compact list of venue IDs/names built from the ref URLs.
// A TeamQuery filter narrows the list to matching entries.
func (c *ESPNClient) LookupVenues(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
	}

	paged, err := c.client.Venues(ctx, cfg.Sport, cfg.League, req.Limit)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	if paged == nil || len(paged.Items) == 0 {
		return nil, ErrNoSportsData
	}

	// Build a simple list from the ref URLs (e.g. .../venues/3615 → id=3615)
	rows := make([][]string, 0, len(paged.Items))
	filter := strings.ToLower(strings.TrimSpace(req.TeamQuery))
	for _, ref := range paged.Items {
		id := venueIDFromRef(ref.Ref)
		if id == "" {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(id), filter) {
			continue
		}
		rows = append(rows, []string{id, ref.Ref})
	}
	if len(rows) == 0 {
		return nil, ErrNoSportsData
	}

	title := fmt.Sprintf("### %s Venues", cfg.DisplayName)
	table := SimpleTable{
		Headers: []string{"Venue ID", "Ref URL"},
		Rows:    rows,
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:      SportsIntentVenues,
		League:      cfg.League,
		LeagueName:  cfg.DisplayName,
		Sport:       cfg.Sport,
		Markdown:    RenderSimpleMarkdown(title, table, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}, nil
}

// venueIDFromRef extracts the trailing numeric ID from a venue ref URL.
// e.g. "http://sports.core.api.espn.com/v2/sports/football/venues/3615" → "3615"
func venueIDFromRef(ref string) string {
	if ref == "" {
		return ""
	}
	idx := strings.LastIndex(ref, "/")
	if idx < 0 {
		return ""
	}
	return ref[idx+1:]
}

// NormalizeVenueStruct converts an espn.Venue to a displayable SimpleTable row
// (used in unit tests; live venue resolution requires individual ref fetches).
func NormalizeVenueStruct(v espn.Venue) []string {
	indoor := "outdoor"
	if v.Indoor {
		indoor = "indoor"
	}
	surface := "turf"
	if v.Grass {
		surface = "grass"
	}
	cap := ""
	if v.Capacity > 0 {
		cap = fmt.Sprintf("%d", v.Capacity)
	}
	city := strings.TrimSpace(v.Address.City + ", " + v.Address.State)
	city = strings.Trim(city, ", ")
	return []string{v.FullName, city, cap, indoor, surface}
}

// ─── Q83–Q84: Power Index ────────────────────────────────────────────────────

// LookupPowerIndex returns the season Power Index leaderboard (FPI/BPI/SP+).
func (c *ESPNClient) LookupPowerIndex(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		// Default to CFB for FPI
		var cfgOk bool
		cfg, cfgOk = leagueConfigByLeague(espn.LeagueCollegeFootball)
		if !cfgOk {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
		}
	}
	raw, err := c.client.PowerIndexLeaders(ctx, cfg.Sport, cfg.League, req.Season)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	title := fmt.Sprintf("### %s Power Index", cfg.DisplayName)
	if req.Season > 0 {
		title = fmt.Sprintf("### %s Power Index (%d)", cfg.DisplayName, req.Season)
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:      SportsIntentPowerIndex,
		League:      cfg.League,
		LeagueName:  cfg.DisplayName,
		Sport:       cfg.Sport,
		Markdown:    RenderSimpleMarkdown(title, table, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}, nil
}

// ─── Q85–Q86: Recruits / Recruiting Class ────────────────────────────────────

// LookupRecruits returns CFB recruit rankings for a season.
// If TeamQuery is set, RecruitingClass is fetched for that team instead.
func (c *ESPNClient) LookupRecruits(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	cfg, ok := leagueConfigForRequest(req)
	if !ok {
		var cfgOk bool
		cfg, cfgOk = leagueConfigByLeague(espn.LeagueCollegeFootball)
		if !cfgOk {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedLeague, req.League)
		}
	}

	var (
		raw   []byte
		title string
		err   error
	)

	teamID := strings.TrimSpace(req.TeamQuery)
	if teamID != "" {
		// Team-specific recruiting class
		raw, err = c.client.RecruitingClass(ctx, cfg.League, req.Season, teamID)
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		title = fmt.Sprintf("### %s Recruiting Class", teamID)
		if req.Season > 0 {
			title = fmt.Sprintf("### %s Recruiting Class (%d)", teamID, req.Season)
		}
	} else {
		// League-wide recruit rankings
		lim := req.Limit
		if lim <= 0 {
			lim = 100
		}
		raw, err = c.client.Recruits(ctx, cfg.League, req.Season, lim)
		if err != nil {
			return nil, wrapESPNError(ctx, err)
		}
		title = fmt.Sprintf("### %s Top Recruits", cfg.DisplayName)
		if req.Season > 0 {
			title = fmt.Sprintf("### %s Top Recruits (%d)", cfg.DisplayName, req.Season)
		}
	}

	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:      SportsIntentRecruits,
		League:      cfg.League,
		LeagueName:  cfg.DisplayName,
		Sport:       cfg.Sport,
		Markdown:    RenderSimpleMarkdown(title, table, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}, nil
}

// ─── Q87: Bracketology ──────────────────────────────────────────────────────

// bracketologyTournamentID is ESPN's Men's NCAAM tournament ID.
const bracketologyTournamentID = "22"

// LookupBracketology returns NCAA tournament bracket projections.
func (c *ESPNClient) LookupBracketology(ctx context.Context, req SportsRequest) (*SportsLookupResult, error) {
	raw, err := c.client.Bracketology(ctx, bracketologyTournamentID, req.Season, 0)
	if err != nil {
		return nil, wrapESPNError(ctx, err)
	}
	table := rawJSONTable(raw, req.Limit)
	if len(table.Rows) == 0 {
		return nil, ErrNoSportsData
	}
	title := "### NCAA Tournament Bracketology"
	if req.Season > 0 {
		title = fmt.Sprintf("### NCAA Tournament Bracketology (%d)", req.Season)
	}
	retrievedAt := c.timeNow()
	return &SportsLookupResult{
		Intent:      SportsIntentBracketology,
		Markdown:    RenderSimpleMarkdown(title, table, retrievedAt),
		Source:      SourceESPN,
		RetrievedAt: retrievedAt,
		RenderMode:  renderMode(req),
	}, nil
}
