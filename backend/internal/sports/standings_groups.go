package sports

import (
	"strconv"
	"strings"

	espn "github.com/chinmaykhachane/espn-go"
)

type standingsTableStyle string

const (
	standingsTablePro     standingsTableStyle = "pro"
	standingsTableHockey  standingsTableStyle = "hockey"
	standingsTableSoccer  standingsTableStyle = "soccer"
	standingsTableCricket standingsTableStyle = "cricket"
)

type standingsSectionID struct {
	Parent  string
	Section string
}

type standingsSection struct {
	Parent  string
	Section string
	Rows    []StandingsRow
}

type standingsGrouping struct {
	FallbackParent       string
	FallbackSection      string
	Order                []standingsSectionID
	TeamSections         map[string]standingsSectionID
	TableStyle           standingsTableStyle
	RecalculateGamesBack bool
}

func standingsGroupingForLeague(cfg LeagueConfig) (standingsGrouping, bool) {
	switch strings.ToLower(strings.TrimSpace(cfg.League)) {
	case espn.LeagueMLB:
		return standingsGrouping{
			FallbackParent:       "MLB",
			FallbackSection:      "Standings",
			Order:                convertMLBDivisionOrder(),
			TeamSections:         convertMLBTeamDivisions(),
			TableStyle:           standingsTablePro,
			RecalculateGamesBack: true,
		}, true
	case espn.LeagueNFL:
		return standingsGrouping{
			FallbackParent:       "NFL",
			FallbackSection:      "Standings",
			Order:                nflDivisionOrder,
			TeamSections:         nflTeamDivisions,
			TableStyle:           standingsTablePro,
			RecalculateGamesBack: true,
		}, true
	case espn.LeagueNBA:
		return standingsGrouping{
			FallbackParent:       "NBA",
			FallbackSection:      "Standings",
			Order:                nbaDivisionOrder,
			TeamSections:         nbaTeamDivisions,
			TableStyle:           standingsTablePro,
			RecalculateGamesBack: true,
		}, true
	case espn.LeagueNHL:
		return standingsGrouping{
			FallbackParent:  "NHL",
			FallbackSection: "Standings",
			Order:           nhlDivisionOrder,
			TeamSections:    nhlTeamDivisions,
			TableStyle:      standingsTableHockey,
		}, true
	case espn.LeagueMLS:
		return standingsGrouping{
			FallbackParent:  "MLS",
			FallbackSection: "Standings",
			Order:           mlsConferenceOrder,
			TeamSections:    mlsTeamConferences,
			TableStyle:      standingsTableSoccer,
		}, true
	default:
		return standingsGrouping{}, false
	}
}

func writeStructuredStandingsMarkdown(b *strings.Builder, rows []StandingsRow, mode SportsRenderMode, grouping standingsGrouping) {
	sections := structuredStandingsSections(rows, grouping)
	currentParent := ""
	for i, section := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		if section.Parent != "" && section.Parent != currentParent {
			currentParent = section.Parent
			b.WriteString("#### ")
			b.WriteString(escapeMarkdownCell(section.Parent))
			b.WriteString("\n\n")
		}
		if section.Section != "" && !strings.EqualFold(section.Section, section.Parent) {
			if section.Parent == "" {
				b.WriteString("#### ")
			} else {
				b.WriteString("##### ")
			}
			b.WriteString(escapeMarkdownCell(section.Section))
			b.WriteString("\n\n")
		}
		b.WriteString(renderStructuredStandingsTable(section.Rows, mode, grouping.TableStyle))
	}
}

func writeStructuredStandingsHTML(b *strings.Builder, rows []StandingsRow, grouping standingsGrouping) {
	sections := structuredStandingsSections(rows, grouping)
	currentParent := ""
	for _, section := range sections {
		if section.Parent != "" && section.Parent != currentParent {
			currentParent = section.Parent
			b.WriteString("<h4>")
			b.WriteString(escapeHTML(section.Parent))
			b.WriteString("</h4>\n")
		}
		if section.Section != "" && !strings.EqualFold(section.Section, section.Parent) {
			if section.Parent == "" {
				b.WriteString("<h4>")
				b.WriteString(escapeHTML(section.Section))
				b.WriteString("</h4>\n")
			} else {
				b.WriteString("<h5>")
				b.WriteString(escapeHTML(section.Section))
				b.WriteString("</h5>\n")
			}
		}
		writeHTMLTable(b, structuredHTMLHeaders(grouping.TableStyle), structuredRowsHTML(section.Rows, grouping.TableStyle))
	}
}

func structuredStandingsSections(rows []StandingsRow, grouping standingsGrouping) []standingsSection {
	buckets := map[standingsSectionID][]StandingsRow{}
	var fallbackRows []StandingsRow
	for _, row := range rows {
		id, ok := standingsSectionForRow(row, grouping)
		if !ok {
			fallbackRows = append(fallbackRows, row)
			continue
		}
		buckets[id] = append(buckets[id], row)
	}

	sections := make([]standingsSection, 0, len(grouping.Order)+1)
	for _, id := range grouping.Order {
		sectionRows := buckets[id]
		if len(sectionRows) == 0 {
			continue
		}
		sections = append(sections, standingsSection{
			Parent:  id.Parent,
			Section: id.Section,
			Rows:    normalizeStructuredRows(sectionRows, grouping),
		})
	}
	if len(fallbackRows) > 0 || len(sections) == 0 {
		sections = append(sections, standingsSection{
			Parent:  grouping.FallbackParent,
			Section: grouping.FallbackSection,
			Rows:    normalizeStructuredRows(firstNonEmptyRows(fallbackRows, rows), grouping),
		})
	}
	return sections
}

func standingsSectionForRow(row StandingsRow, grouping standingsGrouping) (standingsSectionID, bool) {
	if id, ok := standingsSectionFromGroup(row.Group, grouping.Order); ok {
		return id, true
	}
	team := standingsTeam(row)
	fields := []string{
		row.Abbr,
		row.Team,
		team.Abbreviation,
		team.DisplayName,
		team.ShortName,
		team.Location + " " + team.ShortName,
	}
	for _, field := range fields {
		if id, ok := grouping.TeamSections[compactSportsKey(field)]; ok {
			return id, true
		}
	}
	return standingsSectionID{}, false
}

func standingsSectionFromGroup(group string, order []standingsSectionID) (standingsSectionID, bool) {
	norm := compactSportsKey(group)
	if norm == "" {
		return standingsSectionID{}, false
	}
	for _, id := range order {
		parent := compactSportsKey(id.Parent)
		section := compactSportsKey(id.Section)
		if section == "" || !strings.Contains(norm, section) {
			continue
		}
		if parent == "" || strings.Contains(norm, parent) || strings.Contains(section, parent) {
			return id, true
		}
	}
	return standingsSectionID{}, false
}

func normalizeStructuredRows(rows []StandingsRow, grouping standingsGrouping) []StandingsRow {
	out := make([]StandingsRow, len(rows))
	copy(out, rows)
	for i := range out {
		out[i].Rank = i + 1
		if grouping.RecalculateGamesBack {
			out[i].GamesBack = gamesBackFromLeader(out, i)
		}
	}
	return out
}

func renderStructuredStandingsTable(rows []StandingsRow, mode SportsRenderMode, style standingsTableStyle) string {
	return renderTable(structuredHeaders(style), structuredSeparators(style), structuredRows(rows, mode, style))
}

func structuredRows(rows []StandingsRow, mode SportsRenderMode, style standingsTableStyle) [][]string {
	out := make([][]string, 0, len(rows))
	for i, row := range rows {
		switch style {
		case standingsTableSoccer:
			out = append(out, []string{
				standingsRankCell(row, i),
				emptyAsDash(renderTeamCell(standingsTeam(row), mode)),
				emptyAsDash(row.GamesPlayed),
				emptyAsDash(row.Wins),
				emptyAsDash(row.Draws),
				emptyAsDash(row.Losses),
				emptyAsDash(row.Points),
				emptyAsDash(firstNonEmpty(row.GoalDiff, row.GoalDifferential)),
			})
		case standingsTableHockey:
			out = append(out, []string{
				standingsRankCell(row, i),
				emptyAsDash(renderTeamCell(standingsTeam(row), mode)),
				emptyAsDash(row.GamesPlayed),
				emptyAsDash(row.Wins),
				emptyAsDash(row.Losses),
				emptyAsDash(row.Ties),
				emptyAsDash(row.Points),
				emptyAsDash(row.Pct),
				emptyAsDash(row.Streak),
				emptyAsDash(row.LastTen),
			})
		case standingsTableCricket:
			out = append(out, []string{
				standingsRankCell(row, i),
				emptyAsDash(renderTeamCell(standingsTeam(row), mode)),
				emptyAsDash(row.GamesPlayed),
				emptyAsDash(row.Wins),
				emptyAsDash(row.Losses),
				emptyAsDash(row.Ties),
				emptyAsDash(row.NoResult),
				emptyAsDash(row.Points),
				emptyAsDash(row.NetRunRate),
				emptyAsDash(row.For),
				emptyAsDash(row.Against),
			})
		default:
			out = append(out, []string{
				standingsRankCell(row, i),
				emptyAsDash(renderTeamCell(standingsTeam(row), mode)),
				emptyAsDash(row.Wins),
				emptyAsDash(row.Losses),
				emptyAsDash(row.Pct),
				emptyAsDash(row.Streak),
				emptyAsDash(row.LastTen),
				emptyAsDash(row.GamesBack),
			})
		}
	}
	return out
}

func structuredRowsHTML(rows []StandingsRow, style standingsTableStyle) [][]htmlCell {
	tableRows := structuredRows(rows, SportsRenderHTMLMarkdown, style)
	alignments := structuredAlignments(style)
	out := make([][]htmlCell, 0, len(tableRows))
	for _, row := range tableRows {
		cells := make([]htmlCell, 0, len(row))
		for i, cell := range row {
			text := escapeHTML(cell)
			if i == 1 {
				text = cell
			}
			align := ""
			if i < len(alignments) {
				align = alignments[i]
			}
			cells = append(cells, htmlCell{text: text, align: align})
		}
		out = append(out, cells)
	}
	return out
}

func structuredHeaders(style standingsTableStyle) []string {
	switch style {
	case standingsTableSoccer:
		return []string{"Rank", "Club", "GP", "W", "D", "L", "Pts", "GD"}
	case standingsTableHockey:
		return []string{"Rank", "Team", "GP", "W", "L", "OT", "Pts", "Pct", "Strk", "L10"}
	case standingsTableCricket:
		return []string{"Rank", "Team", "M", "W", "L", "T", "N/R", "PT", "NRR", "For", "Against"}
	default:
		return []string{"Rank", "Team", "W", "L", "Pct", "Strk", "L10", "GB"}
	}
}

func structuredSeparators(style standingsTableStyle) []string {
	switch style {
	case standingsTableSoccer:
		return []string{"---:", "---", "---:", "---:", "---:", "---:", "---:", "---:"}
	case standingsTableHockey:
		return []string{"---:", "---", "---:", "---:", "---:", "---:", "---:", "---:", "---", "---"}
	case standingsTableCricket:
		return []string{"---:", "---", "---:", "---:", "---:", "---:", "---:", "---:", "---:", "---:", "---:"}
	default:
		return []string{"---:", "---", "---:", "---:", "---:", "---", "---", "---:"}
	}
}

func structuredAlignments(style standingsTableStyle) []string {
	switch style {
	case standingsTableSoccer:
		return []string{"right", "left", "right", "right", "right", "right", "right", "right"}
	case standingsTableHockey:
		return []string{"right", "left", "right", "right", "right", "right", "right", "right", "left", "left"}
	case standingsTableCricket:
		return []string{"right", "left", "right", "right", "right", "right", "right", "right", "right", "right", "right"}
	default:
		return []string{"right", "left", "right", "right", "right", "left", "left", "right"}
	}
}

func structuredHTMLHeaders(style standingsTableStyle) []htmlHeader {
	headers := structuredHeaders(style)
	alignments := structuredAlignments(style)
	out := make([]htmlHeader, 0, len(headers))
	for i, header := range headers {
		align := ""
		if i < len(alignments) {
			align = alignments[i]
		}
		out = append(out, htmlHeader{Text: header, Align: align})
	}
	return out
}

func firstNonEmptyRows(primary, fallback []StandingsRow) []StandingsRow {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

func gamesBackFromLeader(rows []StandingsRow, index int) string {
	if index <= 0 || len(rows) == 0 {
		return "—"
	}
	leaderWins, okWins := parseStandingsNumber(rows[0].Wins)
	leaderLosses, okLosses := parseStandingsNumber(rows[0].Losses)
	teamWins, okTeamWins := parseStandingsNumber(rows[index].Wins)
	teamLosses, okTeamLosses := parseStandingsNumber(rows[index].Losses)
	if !okWins || !okLosses || !okTeamWins || !okTeamLosses {
		return emptyAsDash(rows[index].GamesBack)
	}
	gb := ((leaderWins - teamWins) + (teamLosses - leaderLosses)) / 2
	if gb <= 0 {
		return "—"
	}
	return strconv.FormatFloat(gb, 'f', 1, 64)
}

func convertMLBDivisionOrder() []standingsSectionID {
	out := make([]standingsSectionID, 0, len(mlbDivisionOrder))
	for _, id := range mlbDivisionOrder {
		out = append(out, standingsSectionID{Parent: id.League, Section: id.Division})
	}
	return out
}

func convertMLBTeamDivisions() map[string]standingsSectionID {
	out := make(map[string]standingsSectionID, len(mlbTeamDivisions))
	for key, id := range mlbTeamDivisions {
		out[key] = standingsSectionID{Parent: id.League, Section: id.Division}
	}
	return out
}

var nflDivisionOrder = []standingsSectionID{
	{Parent: "AFC", Section: "East"},
	{Parent: "AFC", Section: "North"},
	{Parent: "AFC", Section: "South"},
	{Parent: "AFC", Section: "West"},
	{Parent: "NFC", Section: "East"},
	{Parent: "NFC", Section: "North"},
	{Parent: "NFC", Section: "South"},
	{Parent: "NFC", Section: "West"},
}

var nflTeamDivisions = map[string]standingsSectionID{
	"buf": {Parent: "AFC", Section: "East"}, "buffalobills": {Parent: "AFC", Section: "East"},
	"mia": {Parent: "AFC", Section: "East"}, "miamidolphins": {Parent: "AFC", Section: "East"},
	"ne": {Parent: "AFC", Section: "East"}, "newenglandpatriots": {Parent: "AFC", Section: "East"},
	"nyj": {Parent: "AFC", Section: "East"}, "newyorkjets": {Parent: "AFC", Section: "East"},
	"bal": {Parent: "AFC", Section: "North"}, "baltimoreravens": {Parent: "AFC", Section: "North"},
	"cin": {Parent: "AFC", Section: "North"}, "cincinnatibengals": {Parent: "AFC", Section: "North"},
	"cle": {Parent: "AFC", Section: "North"}, "clevelandbrowns": {Parent: "AFC", Section: "North"},
	"pit": {Parent: "AFC", Section: "North"}, "pittsburghsteelers": {Parent: "AFC", Section: "North"},
	"hou": {Parent: "AFC", Section: "South"}, "houstontexans": {Parent: "AFC", Section: "South"},
	"ind": {Parent: "AFC", Section: "South"}, "indianapoliscolts": {Parent: "AFC", Section: "South"},
	"jax": {Parent: "AFC", Section: "South"}, "jacksonvillejaguars": {Parent: "AFC", Section: "South"},
	"ten": {Parent: "AFC", Section: "South"}, "tennesseetitans": {Parent: "AFC", Section: "South"},
	"den": {Parent: "AFC", Section: "West"}, "denverbroncos": {Parent: "AFC", Section: "West"},
	"kc": {Parent: "AFC", Section: "West"}, "kansascitychiefs": {Parent: "AFC", Section: "West"},
	"lv": {Parent: "AFC", Section: "West"}, "lasvegasraiders": {Parent: "AFC", Section: "West"},
	"lac": {Parent: "AFC", Section: "West"}, "losangeleschargers": {Parent: "AFC", Section: "West"},
	"dal": {Parent: "NFC", Section: "East"}, "dallascowboys": {Parent: "NFC", Section: "East"},
	"nyg": {Parent: "NFC", Section: "East"}, "newyorkgiants": {Parent: "NFC", Section: "East"},
	"phi": {Parent: "NFC", Section: "East"}, "philadelphiaeagles": {Parent: "NFC", Section: "East"},
	"wsh": {Parent: "NFC", Section: "East"}, "was": {Parent: "NFC", Section: "East"}, "washingtoncommanders": {Parent: "NFC", Section: "East"},
	"chi": {Parent: "NFC", Section: "North"}, "chicagobears": {Parent: "NFC", Section: "North"},
	"det": {Parent: "NFC", Section: "North"}, "detroitlions": {Parent: "NFC", Section: "North"},
	"gb": {Parent: "NFC", Section: "North"}, "greenbaypackers": {Parent: "NFC", Section: "North"},
	"min": {Parent: "NFC", Section: "North"}, "minnesotavikings": {Parent: "NFC", Section: "North"},
	"atl": {Parent: "NFC", Section: "South"}, "atlantafalcons": {Parent: "NFC", Section: "South"},
	"car": {Parent: "NFC", Section: "South"}, "carolinapanthers": {Parent: "NFC", Section: "South"},
	"no": {Parent: "NFC", Section: "South"}, "neworleanssaints": {Parent: "NFC", Section: "South"},
	"tb": {Parent: "NFC", Section: "South"}, "tampabaybuccaneers": {Parent: "NFC", Section: "South"},
	"ari": {Parent: "NFC", Section: "West"}, "arizonacardinals": {Parent: "NFC", Section: "West"},
	"lar": {Parent: "NFC", Section: "West"}, "losangelesrams": {Parent: "NFC", Section: "West"},
	"sf": {Parent: "NFC", Section: "West"}, "sfo": {Parent: "NFC", Section: "West"}, "sanfrancisco49ers": {Parent: "NFC", Section: "West"},
	"sea": {Parent: "NFC", Section: "West"}, "seattleseahawks": {Parent: "NFC", Section: "West"},
}

var nbaDivisionOrder = []standingsSectionID{
	{Parent: "Eastern Conference", Section: "Atlantic"},
	{Parent: "Eastern Conference", Section: "Central"},
	{Parent: "Eastern Conference", Section: "Southeast"},
	{Parent: "Western Conference", Section: "Northwest"},
	{Parent: "Western Conference", Section: "Pacific"},
	{Parent: "Western Conference", Section: "Southwest"},
}

var nbaTeamDivisions = map[string]standingsSectionID{
	"bos": {Parent: "Eastern Conference", Section: "Atlantic"}, "bostonceltics": {Parent: "Eastern Conference", Section: "Atlantic"},
	"bkn": {Parent: "Eastern Conference", Section: "Atlantic"}, "brooklynnets": {Parent: "Eastern Conference", Section: "Atlantic"},
	"ny": {Parent: "Eastern Conference", Section: "Atlantic"}, "nyk": {Parent: "Eastern Conference", Section: "Atlantic"}, "newyorkknicks": {Parent: "Eastern Conference", Section: "Atlantic"},
	"phi": {Parent: "Eastern Conference", Section: "Atlantic"}, "philadelphia76ers": {Parent: "Eastern Conference", Section: "Atlantic"},
	"tor": {Parent: "Eastern Conference", Section: "Atlantic"}, "torontoraptors": {Parent: "Eastern Conference", Section: "Atlantic"},
	"chi": {Parent: "Eastern Conference", Section: "Central"}, "chicagobulls": {Parent: "Eastern Conference", Section: "Central"},
	"cle": {Parent: "Eastern Conference", Section: "Central"}, "clevelandcavaliers": {Parent: "Eastern Conference", Section: "Central"},
	"det": {Parent: "Eastern Conference", Section: "Central"}, "detroitpistons": {Parent: "Eastern Conference", Section: "Central"},
	"ind": {Parent: "Eastern Conference", Section: "Central"}, "indianapacers": {Parent: "Eastern Conference", Section: "Central"},
	"mil": {Parent: "Eastern Conference", Section: "Central"}, "milwaukeebucks": {Parent: "Eastern Conference", Section: "Central"},
	"atl": {Parent: "Eastern Conference", Section: "Southeast"}, "atlantahawks": {Parent: "Eastern Conference", Section: "Southeast"},
	"cha": {Parent: "Eastern Conference", Section: "Southeast"}, "charlottehornets": {Parent: "Eastern Conference", Section: "Southeast"},
	"mia": {Parent: "Eastern Conference", Section: "Southeast"}, "miamiheat": {Parent: "Eastern Conference", Section: "Southeast"},
	"orl": {Parent: "Eastern Conference", Section: "Southeast"}, "orlandomagic": {Parent: "Eastern Conference", Section: "Southeast"},
	"wsh": {Parent: "Eastern Conference", Section: "Southeast"}, "was": {Parent: "Eastern Conference", Section: "Southeast"}, "washingtonwizards": {Parent: "Eastern Conference", Section: "Southeast"},
	"den": {Parent: "Western Conference", Section: "Northwest"}, "denvernuggets": {Parent: "Western Conference", Section: "Northwest"},
	"min": {Parent: "Western Conference", Section: "Northwest"}, "minnesotatimberwolves": {Parent: "Western Conference", Section: "Northwest"},
	"okc": {Parent: "Western Conference", Section: "Northwest"}, "oklahomacitythunder": {Parent: "Western Conference", Section: "Northwest"},
	"por": {Parent: "Western Conference", Section: "Northwest"}, "portlandtrailblazers": {Parent: "Western Conference", Section: "Northwest"},
	"uta": {Parent: "Western Conference", Section: "Northwest"}, "utahjazz": {Parent: "Western Conference", Section: "Northwest"},
	"gs": {Parent: "Western Conference", Section: "Pacific"}, "gsw": {Parent: "Western Conference", Section: "Pacific"}, "goldenstatewarriors": {Parent: "Western Conference", Section: "Pacific"},
	"lac": {Parent: "Western Conference", Section: "Pacific"}, "losangelesclippers": {Parent: "Western Conference", Section: "Pacific"},
	"lal": {Parent: "Western Conference", Section: "Pacific"}, "losangeleslakers": {Parent: "Western Conference", Section: "Pacific"},
	"phx": {Parent: "Western Conference", Section: "Pacific"}, "phoenixsuns": {Parent: "Western Conference", Section: "Pacific"},
	"sac": {Parent: "Western Conference", Section: "Pacific"}, "sacramentokings": {Parent: "Western Conference", Section: "Pacific"},
	"dal": {Parent: "Western Conference", Section: "Southwest"}, "dallasmavericks": {Parent: "Western Conference", Section: "Southwest"},
	"hou": {Parent: "Western Conference", Section: "Southwest"}, "houstonrockets": {Parent: "Western Conference", Section: "Southwest"},
	"mem": {Parent: "Western Conference", Section: "Southwest"}, "memphisgrizzlies": {Parent: "Western Conference", Section: "Southwest"},
	"no": {Parent: "Western Conference", Section: "Southwest"}, "nop": {Parent: "Western Conference", Section: "Southwest"}, "neworleanspelicans": {Parent: "Western Conference", Section: "Southwest"},
	"sa": {Parent: "Western Conference", Section: "Southwest"}, "sas": {Parent: "Western Conference", Section: "Southwest"}, "sanantoniospurs": {Parent: "Western Conference", Section: "Southwest"},
}

var nhlDivisionOrder = []standingsSectionID{
	{Parent: "Eastern Conference", Section: "Atlantic"},
	{Parent: "Eastern Conference", Section: "Metropolitan"},
	{Parent: "Western Conference", Section: "Central"},
	{Parent: "Western Conference", Section: "Pacific"},
}

var nhlTeamDivisions = map[string]standingsSectionID{
	"bos": {Parent: "Eastern Conference", Section: "Atlantic"}, "bostonbruins": {Parent: "Eastern Conference", Section: "Atlantic"},
	"buf": {Parent: "Eastern Conference", Section: "Atlantic"}, "buffalosabres": {Parent: "Eastern Conference", Section: "Atlantic"},
	"det": {Parent: "Eastern Conference", Section: "Atlantic"}, "detroitredwings": {Parent: "Eastern Conference", Section: "Atlantic"},
	"fla": {Parent: "Eastern Conference", Section: "Atlantic"}, "floridapanthers": {Parent: "Eastern Conference", Section: "Atlantic"},
	"mtl": {Parent: "Eastern Conference", Section: "Atlantic"}, "montrealcanadiens": {Parent: "Eastern Conference", Section: "Atlantic"},
	"ott": {Parent: "Eastern Conference", Section: "Atlantic"}, "ottawasenators": {Parent: "Eastern Conference", Section: "Atlantic"},
	"tb": {Parent: "Eastern Conference", Section: "Atlantic"}, "tbl": {Parent: "Eastern Conference", Section: "Atlantic"}, "tampabaylightning": {Parent: "Eastern Conference", Section: "Atlantic"},
	"tor": {Parent: "Eastern Conference", Section: "Atlantic"}, "torontomapleleafs": {Parent: "Eastern Conference", Section: "Atlantic"},
	"car": {Parent: "Eastern Conference", Section: "Metropolitan"}, "carolinahurricanes": {Parent: "Eastern Conference", Section: "Metropolitan"},
	"cbj": {Parent: "Eastern Conference", Section: "Metropolitan"}, "columbusbluejackets": {Parent: "Eastern Conference", Section: "Metropolitan"},
	"nj": {Parent: "Eastern Conference", Section: "Metropolitan"}, "njd": {Parent: "Eastern Conference", Section: "Metropolitan"}, "newjerseydevils": {Parent: "Eastern Conference", Section: "Metropolitan"},
	"nyi": {Parent: "Eastern Conference", Section: "Metropolitan"}, "newyorkislanders": {Parent: "Eastern Conference", Section: "Metropolitan"},
	"nyr": {Parent: "Eastern Conference", Section: "Metropolitan"}, "newyorkrangers": {Parent: "Eastern Conference", Section: "Metropolitan"},
	"phi": {Parent: "Eastern Conference", Section: "Metropolitan"}, "philadelphiaflyers": {Parent: "Eastern Conference", Section: "Metropolitan"},
	"pit": {Parent: "Eastern Conference", Section: "Metropolitan"}, "pittsburghpenguins": {Parent: "Eastern Conference", Section: "Metropolitan"},
	"wsh": {Parent: "Eastern Conference", Section: "Metropolitan"}, "was": {Parent: "Eastern Conference", Section: "Metropolitan"}, "washingtoncapitals": {Parent: "Eastern Conference", Section: "Metropolitan"},
	"chi": {Parent: "Western Conference", Section: "Central"}, "chicagoblackhawks": {Parent: "Western Conference", Section: "Central"},
	"col": {Parent: "Western Conference", Section: "Central"}, "coloradoavalanche": {Parent: "Western Conference", Section: "Central"},
	"dal": {Parent: "Western Conference", Section: "Central"}, "dallasstars": {Parent: "Western Conference", Section: "Central"},
	"min": {Parent: "Western Conference", Section: "Central"}, "minnesotawild": {Parent: "Western Conference", Section: "Central"},
	"nsh": {Parent: "Western Conference", Section: "Central"}, "nashvillepredators": {Parent: "Western Conference", Section: "Central"},
	"stl": {Parent: "Western Conference", Section: "Central"}, "stlouisblues": {Parent: "Western Conference", Section: "Central"},
	"uta": {Parent: "Western Conference", Section: "Central"}, "utahhockeyclub": {Parent: "Western Conference", Section: "Central"}, "utahmammoth": {Parent: "Western Conference", Section: "Central"},
	"wpg": {Parent: "Western Conference", Section: "Central"}, "winnipegjets": {Parent: "Western Conference", Section: "Central"},
	"ana": {Parent: "Western Conference", Section: "Pacific"}, "anaheimducks": {Parent: "Western Conference", Section: "Pacific"},
	"cgy": {Parent: "Western Conference", Section: "Pacific"}, "calgaryflames": {Parent: "Western Conference", Section: "Pacific"},
	"edm": {Parent: "Western Conference", Section: "Pacific"}, "edmontonoilers": {Parent: "Western Conference", Section: "Pacific"},
	"la": {Parent: "Western Conference", Section: "Pacific"}, "lak": {Parent: "Western Conference", Section: "Pacific"}, "losangeleskings": {Parent: "Western Conference", Section: "Pacific"},
	"sj": {Parent: "Western Conference", Section: "Pacific"}, "sjs": {Parent: "Western Conference", Section: "Pacific"}, "sanjosesharks": {Parent: "Western Conference", Section: "Pacific"},
	"sea": {Parent: "Western Conference", Section: "Pacific"}, "seattlekraken": {Parent: "Western Conference", Section: "Pacific"},
	"van": {Parent: "Western Conference", Section: "Pacific"}, "vancouvercanucks": {Parent: "Western Conference", Section: "Pacific"},
	"vgk": {Parent: "Western Conference", Section: "Pacific"}, "vegasgoldenknights": {Parent: "Western Conference", Section: "Pacific"},
}

var mlsConferenceOrder = []standingsSectionID{
	{Section: "Eastern Conference"},
	{Section: "Western Conference"},
}

var mlsTeamConferences = map[string]standingsSectionID{
	"atl": {Section: "Eastern Conference"}, "atlantaunitedfc": {Section: "Eastern Conference"},
	"cha": {Section: "Eastern Conference"}, "clt": {Section: "Eastern Conference"}, "charlottefc": {Section: "Eastern Conference"},
	"chi": {Section: "Eastern Conference"}, "chicagofirefc": {Section: "Eastern Conference"},
	"cin": {Section: "Eastern Conference"}, "fccincinnati": {Section: "Eastern Conference"},
	"clb": {Section: "Eastern Conference"}, "columbuscrew": {Section: "Eastern Conference"},
	"dc": {Section: "Eastern Conference"}, "dcunited": {Section: "Eastern Conference"},
	"mia": {Section: "Eastern Conference"}, "intermiamicf": {Section: "Eastern Conference"},
	"mtl": {Section: "Eastern Conference"}, "cfmontreal": {Section: "Eastern Conference"},
	"ne": {Section: "Eastern Conference"}, "newenglandrevolution": {Section: "Eastern Conference"},
	"nyc": {Section: "Eastern Conference"}, "nycfc": {Section: "Eastern Conference"}, "newyorkcityfc": {Section: "Eastern Conference"},
	"ny": {Section: "Eastern Conference"}, "nyrb": {Section: "Eastern Conference"}, "newyorkredbulls": {Section: "Eastern Conference"},
	"orl": {Section: "Eastern Conference"}, "orlandocitysc": {Section: "Eastern Conference"},
	"phi": {Section: "Eastern Conference"}, "philadelphiaunion": {Section: "Eastern Conference"},
	"tor": {Section: "Eastern Conference"}, "torontofc": {Section: "Eastern Conference"},
	"nsh": {Section: "Eastern Conference"}, "nashvillesc": {Section: "Eastern Conference"},
	"atx": {Section: "Western Conference"}, "austinfc": {Section: "Western Conference"},
	"col": {Section: "Western Conference"}, "coloradorapids": {Section: "Western Conference"},
	"dal": {Section: "Western Conference"}, "fcdallas": {Section: "Western Conference"},
	"hou": {Section: "Western Conference"}, "houstondynamofc": {Section: "Western Conference"},
	"skc": {Section: "Western Conference"}, "sportingkansascity": {Section: "Western Conference"},
	"la": {Section: "Western Conference"}, "lag": {Section: "Western Conference"}, "lagalaxy": {Section: "Western Conference"},
	"lafc": {Section: "Western Conference"}, "losangelesfc": {Section: "Western Conference"},
	"min": {Section: "Western Conference"}, "minnesotaunitedfc": {Section: "Western Conference"},
	"por": {Section: "Western Conference"}, "portlandtimbers": {Section: "Western Conference"},
	"rsl": {Section: "Western Conference"}, "realsaltlake": {Section: "Western Conference"},
	"sd": {Section: "Western Conference"}, "sdfc": {Section: "Western Conference"}, "sandiegofc": {Section: "Western Conference"},
	"sea": {Section: "Western Conference"}, "seattlesoundersfc": {Section: "Western Conference"},
	"sj": {Section: "Western Conference"}, "sanjoseearthquakes": {Section: "Western Conference"},
	"stl": {Section: "Western Conference"}, "stlouiscitysc": {Section: "Western Conference"},
	"van": {Section: "Western Conference"}, "vancouverwhitecaps": {Section: "Western Conference"},
}
