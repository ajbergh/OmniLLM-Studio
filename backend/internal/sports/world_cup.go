package sports

import espn "github.com/chinmaykhachane/espn-go"

// LeagueFIFAWorldCup is ESPN's public scoreboard slug for the men's FIFA World Cup.
const LeagueFIFAWorldCup = "fifa.world"

func init() {
	worldCup := LeagueConfig{
		DisplayName: "FIFA World Cup",
		Sport:       espn.SportSoccer,
		League:      LeagueFIFAWorldCup,
		Aliases: []string{
			"fifa world cup",
			"world cup",
			"2026 world cup",
			"men's world cup",
			"mens world cup",
		},
	}
	leagueConfigs = append([]LeagueConfig{worldCup}, leagueConfigs...)
}
