package sports

import (
	"net/url"
	"strings"

	espn "github.com/chinmaykhachane/espn-go"
)

func renderMode(req SportsRequest) SportsRenderMode {
	switch req.RenderMode {
	case SportsRenderPlainMarkdown, SportsRenderEnhancedMarkdown, SportsRenderHTMLMarkdown:
		return req.RenderMode
	default:
		return DefaultSportsRenderMode
	}
}

func extractTeamIdentityFromCompetitor(c espn.Competitor) TeamIdentity {
	return teamIdentityFromESPNTeam(c.Team)
}

func extractTeamIdentityFromStandingEntry(entry espn.StandingsEntry) TeamIdentity {
	return teamIdentityFromESPNTeam(entry.Team)
}

func teamIdentityFromESPNTeam(team espn.Team) TeamIdentity {
	return TeamIdentity{
		DisplayName:    teamDisplayName(team),
		ShortName:      firstNonEmpty(team.ShortDisplayName, team.Name, team.Nickname),
		Abbreviation:   strings.TrimSpace(team.Abbreviation),
		Location:       strings.TrimSpace(team.Location),
		LogoURL:        firstValidLogoURL(team),
		DarkLogoURL:    darkLogoURL(team),
		PrimaryColor:   normalizeTeamColor(team.Color),
		AlternateColor: normalizeTeamColor(team.AlternateColor),
	}
}

func firstValidLogoURL(team espn.Team) string {
	if url := logoURLFromLogos(team.Logos, false); url != "" {
		return url
	}
	return normalizeLogoURL(team.Logo)
}

func darkLogoURL(team espn.Team) string {
	return logoURLFromLogos(team.Logos, true)
}

func logoURLFromLogos(logos []espn.Logo, preferDark bool) string {
	if len(logos) == 0 {
		return ""
	}

	if preferDark {
		for _, logo := range logos {
			if !logoRelContains(logo.Rel, "dark") {
				continue
			}
			if url := normalizeLogoURL(logo.Href); url != "" {
				return url
			}
		}
		return ""
	}

	for _, logo := range logos {
		if logoRelContains(logo.Rel, "dark") {
			continue
		}
		if logoRelContainsAny(logo.Rel, []string{"default", "full", "logo"}) {
			if url := normalizeLogoURL(logo.Href); url != "" {
				return url
			}
		}
	}
	for _, logo := range logos {
		if logoRelContains(logo.Rel, "dark") {
			continue
		}
		if url := normalizeLogoURL(logo.Href); url != "" {
			return url
		}
	}
	for _, logo := range logos {
		if url := normalizeLogoURL(logo.Href); url != "" {
			return url
		}
	}
	return ""
}

func logoURLFromScoreboard(sb *espn.Scoreboard, cfg LeagueConfig) string {
	if sb != nil {
		for _, league := range sb.Leagues {
			if url := logoURLFromLogos(league.Logos, false); url != "" {
				return url
			}
		}
	}
	return leagueIdentityForConfig(cfg).LogoURL
}

func leagueIdentityForConfig(cfg LeagueConfig) LeagueIdentity {
	display := strings.TrimSpace(cfg.DisplayName)
	abbr := display
	if strings.Contains(display, " ") {
		abbr = strings.ToUpper(cfg.League)
	}
	if abbr == "" {
		abbr = strings.ToUpper(cfg.League)
	}
	return LeagueIdentity{
		League:       cfg.League,
		DisplayName:  display,
		LogoURL:      sanitizeImageURL(leagueLogoURL(cfg.League)),
		Abbreviation: abbr,
	}
}

func leagueLogoURL(league string) string {
	switch strings.ToLower(strings.TrimSpace(league)) {
	case espn.LeagueMLB:
		return "https://a.espncdn.com/i/teamlogos/leagues/500/mlb.png"
	case espn.LeagueNFL:
		return "https://a.espncdn.com/i/teamlogos/leagues/500/nfl.png"
	case espn.LeagueNBA:
		return "https://a.espncdn.com/i/teamlogos/leagues/500/nba.png"
	case espn.LeagueWNBA:
		return "https://a.espncdn.com/i/teamlogos/leagues/500/wnba.png"
	case espn.LeagueNHL:
		return "https://a.espncdn.com/i/teamlogos/leagues/500/nhl.png"
	case espn.LeagueCollegeFootball:
		return "https://a.espncdn.com/i/teamlogos/leagues/500/college-football.png"
	case espn.LeagueMensCollegeBball:
		return "https://a.espncdn.com/i/teamlogos/leagues/500/mens-college-basketball.png"
	case espn.LeagueWomensCollegeBall:
		return "https://a.espncdn.com/i/teamlogos/leagues/500/womens-college-basketball.png"
	case espn.LeagueEPL:
		return "https://a.espncdn.com/i/leaguelogos/soccer/500/23.png"
	case espn.LeagueMLS:
		return "https://a.espncdn.com/i/leaguelogos/soccer/500/19.png"
	default:
		return ""
	}
}

func sanitizeImageURL(raw string) string {
	return normalizeLogoURL(raw)
}

func normalizeLogoURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n\t") {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	if strings.HasPrefix(strings.ToLower(raw), "http://") {
		raw = "https://" + raw[len("http://"):]
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

func logoRelContainsAny(rel []string, values []string) bool {
	for _, value := range values {
		if logoRelContains(rel, value) {
			return true
		}
	}
	return false
}

func logoRelContains(rel []string, value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, item := range rel {
		if strings.Contains(strings.ToLower(strings.TrimSpace(item)), value) {
			return true
		}
	}
	return false
}

func normalizeTeamColor(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "#"))
	if value == "" {
		return ""
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return ""
		}
	}
	return "#" + strings.ToUpper(value)
}

func gameAway(row GameRow) TeamIdentity {
	return ensureTeamIdentity(row.Away, row.AwayTeam, row.AwayAbbr)
}

func gameHome(row GameRow) TeamIdentity {
	return ensureTeamIdentity(row.Home, row.HomeTeam, row.HomeAbbr)
}

func standingsTeam(row StandingsRow) TeamIdentity {
	return ensureTeamIdentity(row.TeamIdentity, row.Team, row.Abbr)
}

func oddsAway(row OddsRow) TeamIdentity {
	return ensureTeamIdentity(row.Away, row.AwayTeam, row.AwayAbbr)
}

func oddsHome(row OddsRow) TeamIdentity {
	return ensureTeamIdentity(row.Home, row.HomeTeam, row.HomeAbbr)
}

func ensureTeamIdentity(team TeamIdentity, displayName, abbreviation string) TeamIdentity {
	if strings.TrimSpace(team.DisplayName) == "" {
		team.DisplayName = strings.TrimSpace(displayName)
	}
	if strings.TrimSpace(team.Abbreviation) == "" {
		team.Abbreviation = strings.TrimSpace(abbreviation)
	}
	if strings.TrimSpace(team.ShortName) == "" {
		team.ShortName = strings.TrimSpace(displayName)
	}
	return team
}
