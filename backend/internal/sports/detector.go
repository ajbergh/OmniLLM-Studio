package sports

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	espn "github.com/chinmaykhachane/espn-go"
)

var exactDatePattern = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
var slashDatePattern = regexp.MustCompile(`\b\d{1,2}/\d{1,2}/\d{4}\b`)
var seasonPattern = regexp.MustCompile(`\b(19|20)\d{2}\b`)
var topLimitPattern = regexp.MustCompile(`\btop\s+(\d{1,3})\b`)

var leagueConfigs = []LeagueConfig{
	{
		DisplayName: "MLB",
		Sport:       espn.SportBaseball,
		League:      espn.LeagueMLB,
		Aliases:     []string{"mlb", "baseball", "major league baseball"},
	},
	{
		DisplayName: "NFL",
		Sport:       espn.SportFootball,
		League:      espn.LeagueNFL,
		Aliases:     []string{"nfl", "football", "pro football"},
	},
	{
		DisplayName: "NBA",
		Sport:       espn.SportBasketball,
		League:      espn.LeagueNBA,
		Aliases:     []string{"nba", "basketball"},
	},
	{
		DisplayName: "WNBA",
		Sport:       espn.SportBasketball,
		League:      espn.LeagueWNBA,
		Aliases:     []string{"wnba"},
	},
	{
		DisplayName: "NHL",
		Sport:       espn.SportHockey,
		League:      espn.LeagueNHL,
		Aliases:     []string{"nhl", "hockey"},
	},
	{
		DisplayName: "College Football",
		Sport:       espn.SportFootball,
		League:      espn.LeagueCollegeFootball,
		Aliases:     []string{"college football", "ncaaf", "cfb"},
	},
	{
		DisplayName: "Men's College Basketball",
		Sport:       espn.SportBasketball,
		League:      espn.LeagueMensCollegeBball,
		Aliases:     []string{"men's college basketball", "mens college basketball", "ncaamb", "college basketball"},
	},
	{
		DisplayName: "Women's College Basketball",
		Sport:       espn.SportBasketball,
		League:      espn.LeagueWomensCollegeBall,
		Aliases:     []string{"women's college basketball", "womens college basketball", "ncaawb"},
	},
	{
		DisplayName: "Premier League",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueEPL,
		Aliases:     []string{"premier league", "epl", "english premier league"},
	},
	{
		DisplayName: "MLS",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueMLS,
		Aliases:     []string{"mls", "major league soccer"},
	},
	{
		DisplayName: "Champions League",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueChampionsLg,
		Aliases:     []string{"champions league", "ucl", "uefa champions league"},
	},
	{
		DisplayName: "La Liga",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueLaLiga,
		Aliases:     []string{"la liga", "laliga", "spanish league", "primera division"},
	},
	{
		DisplayName: "Bundesliga",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueBundesliga,
		Aliases:     []string{"bundesliga", "german league"},
	},
	{
		DisplayName: "Indian Premier League",
		Sport:       espn.SportCricket,
		League:      LeagueIPL,
		Aliases: []string{
			"ipl",
			"ipl cricket",
			"indian premier league",
			"indian premier league cricket",
			"indian premier cricket league",
			"indian premier cricket league cricket",
		},
	},
	// European Soccer (additional leagues)
	{
		DisplayName: "Serie A",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueSerieA,
		Aliases:     []string{"serie a", "serie a calcio", "italian league", "calcio", "italian football"},
	},
	{
		DisplayName: "Ligue 1",
		Sport:       espn.SportSoccer,
		League:      espn.LeagueLigue1,
		Aliases:     []string{"ligue 1", "ligue1", "french league", "ligue un"},
	},
	// Motorsports
	{
		DisplayName: "Formula 1",
		Sport:       espn.SportRacing,
		League:      espn.LeagueF1,
		Aliases:     []string{"f1", "formula 1", "formula one", "formula1", "f1 racing"},
	},
	{
		DisplayName: "NASCAR Cup",
		Sport:       espn.SportRacing,
		League:      espn.LeagueNASCARCup,
		Aliases:     []string{"nascar", "nascar cup", "nascar cup series", "cup series", "stock car"},
	},
	// Golf
	{
		DisplayName: "PGA Tour",
		Sport:       espn.SportGolf,
		League:      espn.LeaguePGA,
		Aliases:     []string{"pga", "pga tour", "golf", "men's golf"},
	},
	// Tennis
	{
		DisplayName: "ATP Tennis",
		Sport:       espn.SportTennis,
		League:      espn.LeagueATP,
		Aliases:     []string{"atp", "atp tennis", "men's tennis", "mens tennis", "atp tour"},
	},
}

type teamAlias struct {
	League    string
	TeamQuery string
	Aliases   []string
}

var teamAliases = []teamAlias{
	{League: espn.LeagueMLB, TeamQuery: "New York Yankees", Aliases: []string{"yankees", "new york yankees", "nyy"}},
	{League: espn.LeagueMLB, TeamQuery: "Chicago Cubs", Aliases: []string{"cubs", "chicago cubs", "chc"}},
	{League: espn.LeagueMLB, TeamQuery: "Boston Red Sox", Aliases: []string{"red sox", "boston red sox"}},
	{League: espn.LeagueMLB, TeamQuery: "Los Angeles Dodgers", Aliases: []string{"dodgers", "los angeles dodgers", "la dodgers", "lad"}},
	{League: espn.LeagueMLB, TeamQuery: "New York Mets", Aliases: []string{"mets", "new york mets", "nym"}},
	{League: espn.LeagueMLB, TeamQuery: "Philadelphia Phillies", Aliases: []string{"phillies", "philadelphia phillies"}},
	{League: espn.LeagueMLB, TeamQuery: "Atlanta Braves", Aliases: []string{"braves", "atlanta braves"}},
	{League: espn.LeagueMLB, TeamQuery: "St. Louis Cardinals", Aliases: []string{"st louis cardinals", "st. louis cardinals"}},
	{League: espn.LeagueMLB, TeamQuery: "San Francisco Giants", Aliases: []string{"san francisco giants", "sf giants", "sfg"}},
	{League: espn.LeagueMLB, TeamQuery: "San Diego Padres", Aliases: []string{"padres", "san diego padres"}},
	{League: espn.LeagueMLB, TeamQuery: "Houston Astros", Aliases: []string{"astros", "houston astros"}},
	{League: espn.LeagueMLB, TeamQuery: "Chicago White Sox", Aliases: []string{"white sox", "chicago white sox"}},
	{League: espn.LeagueMLB, TeamQuery: "Texas Rangers", Aliases: []string{"texas rangers"}},
	{League: espn.LeagueMLB, TeamQuery: "Minnesota Twins", Aliases: []string{"twins", "minnesota twins"}},
	{League: espn.LeagueMLB, TeamQuery: "Toronto Blue Jays", Aliases: []string{"blue jays", "toronto blue jays"}},
	{League: espn.LeagueMLB, TeamQuery: "Tampa Bay Rays", Aliases: []string{"rays", "tampa bay rays"}},
	{League: espn.LeagueMLB, TeamQuery: "Baltimore Orioles", Aliases: []string{"orioles", "baltimore orioles"}},
	{League: espn.LeagueMLB, TeamQuery: "Seattle Mariners", Aliases: []string{"mariners", "seattle mariners"}},
	{League: espn.LeagueMLB, TeamQuery: "Cincinnati Reds", Aliases: []string{"reds", "cincinnati reds"}},
	{League: espn.LeagueMLB, TeamQuery: "Milwaukee Brewers", Aliases: []string{"brewers", "milwaukee brewers"}},
	{League: espn.LeagueMLB, TeamQuery: "Pittsburgh Pirates", Aliases: []string{"pirates", "pittsburgh pirates"}},
	{League: espn.LeagueMLB, TeamQuery: "Colorado Rockies", Aliases: []string{"rockies", "colorado rockies"}},
	{League: espn.LeagueMLB, TeamQuery: "Arizona Diamondbacks", Aliases: []string{"diamondbacks", "arizona diamondbacks"}},
	{League: espn.LeagueMLB, TeamQuery: "Miami Marlins", Aliases: []string{"marlins", "miami marlins"}},
	{League: espn.LeagueMLB, TeamQuery: "Washington Nationals", Aliases: []string{"nationals", "washington nationals"}},
	{League: espn.LeagueMLB, TeamQuery: "Detroit Tigers", Aliases: []string{"tigers", "detroit tigers"}},
	{League: espn.LeagueMLB, TeamQuery: "Oakland Athletics", Aliases: []string{"athletics", "oakland athletics"}},
	{League: espn.LeagueMLB, TeamQuery: "Los Angeles Angels", Aliases: []string{"angels", "los angeles angels", "la angels"}},
	{League: espn.LeagueMLB, TeamQuery: "Cleveland Guardians", Aliases: []string{"guardians", "cleveland guardians"}},
	{League: espn.LeagueMLB, TeamQuery: "Kansas City Royals", Aliases: []string{"royals", "kansas city royals"}},
	{League: espn.LeagueNFL, TeamQuery: "Kansas City Chiefs", Aliases: []string{"chiefs", "kansas city chiefs", "kc chiefs"}},
	{League: espn.LeagueNFL, TeamQuery: "Dallas Cowboys", Aliases: []string{"cowboys", "dallas cowboys"}},
	{League: espn.LeagueNFL, TeamQuery: "Green Bay Packers", Aliases: []string{"packers", "green bay packers"}},
	{League: espn.LeagueNFL, TeamQuery: "San Francisco 49ers", Aliases: []string{"49ers", "niners", "san francisco 49ers", "sf 49ers"}},
	{League: espn.LeagueNFL, TeamQuery: "New England Patriots", Aliases: []string{"patriots", "new england patriots"}},
	{League: espn.LeagueNFL, TeamQuery: "Philadelphia Eagles", Aliases: []string{"eagles", "philadelphia eagles"}},
	{League: espn.LeagueNFL, TeamQuery: "Buffalo Bills", Aliases: []string{"bills", "buffalo bills"}},
	{League: espn.LeagueNFL, TeamQuery: "Chicago Bears", Aliases: []string{"bears", "chicago bears"}},
	{League: espn.LeagueNFL, TeamQuery: "Detroit Lions", Aliases: []string{"lions", "detroit lions"}},
	{League: espn.LeagueNFL, TeamQuery: "Los Angeles Rams", Aliases: []string{"rams", "los angeles rams", "la rams"}},
	{League: espn.LeagueNFL, TeamQuery: "Baltimore Ravens", Aliases: []string{"ravens", "baltimore ravens"}},
	{League: espn.LeagueNFL, TeamQuery: "Pittsburgh Steelers", Aliases: []string{"steelers", "pittsburgh steelers"}},
	{League: espn.LeagueNFL, TeamQuery: "Denver Broncos", Aliases: []string{"broncos", "denver broncos"}},
	{League: espn.LeagueNFL, TeamQuery: "Las Vegas Raiders", Aliases: []string{"raiders", "las vegas raiders"}},
	{League: espn.LeagueNFL, TeamQuery: "Los Angeles Chargers", Aliases: []string{"chargers", "los angeles chargers", "la chargers"}},
	{League: espn.LeagueNFL, TeamQuery: "Seattle Seahawks", Aliases: []string{"seahawks", "seattle seahawks"}},
	{League: espn.LeagueNFL, TeamQuery: "Tampa Bay Buccaneers", Aliases: []string{"buccaneers", "bucs", "tampa bay buccaneers"}},
	{League: espn.LeagueNFL, TeamQuery: "New Orleans Saints", Aliases: []string{"saints", "new orleans saints"}},
	{League: espn.LeagueNFL, TeamQuery: "Cincinnati Bengals", Aliases: []string{"bengals", "cincinnati bengals"}},
	{League: espn.LeagueNFL, TeamQuery: "Minnesota Vikings", Aliases: []string{"vikings", "minnesota vikings"}},
	{League: espn.LeagueNFL, TeamQuery: "New York Giants", Aliases: []string{"new york giants", "ny giants"}},
	{League: espn.LeagueNFL, TeamQuery: "New York Jets", Aliases: []string{"new york jets", "ny jets"}},
	{League: espn.LeagueNFL, TeamQuery: "Miami Dolphins", Aliases: []string{"dolphins", "miami dolphins"}},
	{League: espn.LeagueNFL, TeamQuery: "Houston Texans", Aliases: []string{"texans", "houston texans"}},
	{League: espn.LeagueNFL, TeamQuery: "Indianapolis Colts", Aliases: []string{"colts", "indianapolis colts"}},
	{League: espn.LeagueNFL, TeamQuery: "Cleveland Browns", Aliases: []string{"browns", "cleveland browns"}},
	{League: espn.LeagueNFL, TeamQuery: "Atlanta Falcons", Aliases: []string{"falcons", "atlanta falcons"}},
	{League: espn.LeagueNFL, TeamQuery: "Carolina Panthers", Aliases: []string{"carolina panthers"}},
	{League: espn.LeagueNFL, TeamQuery: "Washington Commanders", Aliases: []string{"commanders", "washington commanders"}},
	{League: espn.LeagueNFL, TeamQuery: "Arizona Cardinals", Aliases: []string{"arizona cardinals"}},
	{League: espn.LeagueNFL, TeamQuery: "Jacksonville Jaguars", Aliases: []string{"jaguars", "jacksonville jaguars"}},
	{League: espn.LeagueNFL, TeamQuery: "Tennessee Titans", Aliases: []string{"titans", "tennessee titans"}},
	{League: espn.LeagueNBA, TeamQuery: "Los Angeles Lakers", Aliases: []string{"lakers", "los angeles lakers", "la lakers"}},
	{League: espn.LeagueNBA, TeamQuery: "Boston Celtics", Aliases: []string{"celtics", "boston celtics"}},
	{League: espn.LeagueNBA, TeamQuery: "Golden State Warriors", Aliases: []string{"warriors", "golden state warriors", "gsw"}},
	{League: espn.LeagueNBA, TeamQuery: "New York Knicks", Aliases: []string{"knicks", "new york knicks", "nicks", "knics"}},
	{League: espn.LeagueNBA, TeamQuery: "Chicago Bulls", Aliases: []string{"bulls", "chicago bulls"}},
	{League: espn.LeagueNBA, TeamQuery: "Miami Heat", Aliases: []string{"heat", "miami heat"}},
	{League: espn.LeagueNBA, TeamQuery: "Denver Nuggets", Aliases: []string{"nuggets", "denver nuggets"}},
	{League: espn.LeagueNBA, TeamQuery: "Dallas Mavericks", Aliases: []string{"mavericks", "mavs", "dallas mavericks"}},
	{League: espn.LeagueNBA, TeamQuery: "Phoenix Suns", Aliases: []string{"suns", "phoenix suns"}},
	{League: espn.LeagueNBA, TeamQuery: "Los Angeles Clippers", Aliases: []string{"clippers", "los angeles clippers", "la clippers"}},
	{League: espn.LeagueNBA, TeamQuery: "Houston Rockets", Aliases: []string{"rockets", "houston rockets"}},
	{League: espn.LeagueNBA, TeamQuery: "Oklahoma City Thunder", Aliases: []string{"thunder", "okc thunder", "oklahoma city thunder"}},
	{League: espn.LeagueNBA, TeamQuery: "Philadelphia 76ers", Aliases: []string{"76ers", "sixers", "philadelphia 76ers"}},
	{League: espn.LeagueNBA, TeamQuery: "Milwaukee Bucks", Aliases: []string{"bucks", "milwaukee bucks"}},
	{League: espn.LeagueNBA, TeamQuery: "Brooklyn Nets", Aliases: []string{"nets", "brooklyn nets"}},
	{League: espn.LeagueNBA, TeamQuery: "Toronto Raptors", Aliases: []string{"raptors", "toronto raptors"}},
	{League: espn.LeagueNBA, TeamQuery: "Sacramento Kings", Aliases: []string{"sacramento kings"}},
	{League: espn.LeagueNBA, TeamQuery: "San Antonio Spurs", Aliases: []string{"spurs", "san antonio spurs"}},
	{League: espn.LeagueNBA, TeamQuery: "Memphis Grizzlies", Aliases: []string{"grizzlies", "memphis grizzlies"}},
	{League: espn.LeagueNBA, TeamQuery: "New Orleans Pelicans", Aliases: []string{"pelicans", "new orleans pelicans"}},
	{League: espn.LeagueNBA, TeamQuery: "Utah Jazz", Aliases: []string{"utah jazz"}},
	{League: espn.LeagueNBA, TeamQuery: "Minnesota Timberwolves", Aliases: []string{"timberwolves", "minnesota timberwolves"}},
	{League: espn.LeagueNBA, TeamQuery: "Cleveland Cavaliers", Aliases: []string{"cavaliers", "cavs", "cleveland cavaliers"}},
	{League: espn.LeagueNBA, TeamQuery: "Indiana Pacers", Aliases: []string{"pacers", "indiana pacers"}},
	{League: espn.LeagueNBA, TeamQuery: "Portland Trail Blazers", Aliases: []string{"trail blazers", "blazers", "portland trail blazers"}},
	{League: espn.LeagueNBA, TeamQuery: "Washington Wizards", Aliases: []string{"wizards", "washington wizards"}},
	{League: espn.LeagueNBA, TeamQuery: "Atlanta Hawks", Aliases: []string{"hawks", "atlanta hawks"}},
	{League: espn.LeagueNBA, TeamQuery: "Orlando Magic", Aliases: []string{"orlando magic"}},
	{League: espn.LeagueNBA, TeamQuery: "Charlotte Hornets", Aliases: []string{"hornets", "charlotte hornets"}},
	{League: espn.LeagueNBA, TeamQuery: "Detroit Pistons", Aliases: []string{"pistons", "detroit pistons"}},
	{League: espn.LeagueNHL, TeamQuery: "Boston Bruins", Aliases: []string{"bruins", "boston bruins"}},
	{League: espn.LeagueNHL, TeamQuery: "Toronto Maple Leafs", Aliases: []string{"maple leafs", "leafs", "toronto maple leafs"}},
	{League: espn.LeagueNHL, TeamQuery: "New York Rangers", Aliases: []string{"new york rangers", "ny rangers"}},
	{League: espn.LeagueNHL, TeamQuery: "Chicago Blackhawks", Aliases: []string{"blackhawks", "chicago blackhawks"}},
	{League: espn.LeagueNHL, TeamQuery: "Colorado Avalanche", Aliases: []string{"avalanche", "avs", "colorado avalanche"}},
	{League: espn.LeagueNHL, TeamQuery: "Tampa Bay Lightning", Aliases: []string{"lightning", "tampa bay lightning"}},
	{League: espn.LeagueNHL, TeamQuery: "Florida Panthers", Aliases: []string{"florida panthers"}},
	{League: espn.LeagueNHL, TeamQuery: "Vegas Golden Knights", Aliases: []string{"golden knights", "vegas golden knights", "vgk"}},
	{League: espn.LeagueNHL, TeamQuery: "Pittsburgh Penguins", Aliases: []string{"penguins", "pittsburgh penguins", "pens"}},
	{League: espn.LeagueNHL, TeamQuery: "Washington Capitals", Aliases: []string{"capitals", "caps", "washington capitals"}},
	{League: espn.LeagueNHL, TeamQuery: "Carolina Hurricanes", Aliases: []string{"hurricanes", "canes", "carolina hurricanes"}},
	{League: espn.LeagueNHL, TeamQuery: "Montreal Canadiens", Aliases: []string{"canadiens", "habs", "montreal canadiens"}},
	{League: espn.LeagueNHL, TeamQuery: "Edmonton Oilers", Aliases: []string{"oilers", "edmonton oilers"}},
	{League: espn.LeagueNHL, TeamQuery: "Calgary Flames", Aliases: []string{"flames", "calgary flames"}},
	{League: espn.LeagueNHL, TeamQuery: "Vancouver Canucks", Aliases: []string{"canucks", "vancouver canucks"}},
	{League: espn.LeagueNHL, TeamQuery: "Winnipeg Jets", Aliases: []string{"winnipeg jets"}},
	{League: espn.LeagueNHL, TeamQuery: "Dallas Stars", Aliases: []string{"stars", "dallas stars"}},
	{League: espn.LeagueNHL, TeamQuery: "Nashville Predators", Aliases: []string{"predators", "preds", "nashville predators"}},
	{League: espn.LeagueNHL, TeamQuery: "New Jersey Devils", Aliases: []string{"devils", "new jersey devils"}},
	{League: espn.LeagueNHL, TeamQuery: "Detroit Red Wings", Aliases: []string{"red wings", "detroit red wings"}},
	{League: espn.LeagueNHL, TeamQuery: "Seattle Kraken", Aliases: []string{"kraken", "seattle kraken"}},
	{League: espn.LeagueNHL, TeamQuery: "San Jose Sharks", Aliases: []string{"sharks", "san jose sharks"}},
	{League: espn.LeagueNHL, TeamQuery: "Ottawa Senators", Aliases: []string{"ottawa senators", "sens"}},
	{League: espn.LeagueNHL, TeamQuery: "Buffalo Sabres", Aliases: []string{"sabres", "buffalo sabres"}},
	{League: espn.LeagueNHL, TeamQuery: "St. Louis Blues", Aliases: []string{"st louis blues", "st. louis blues"}},
	{League: espn.LeagueNHL, TeamQuery: "Columbus Blue Jackets", Aliases: []string{"blue jackets", "columbus blue jackets"}},
	{League: espn.LeagueNHL, TeamQuery: "Philadelphia Flyers", Aliases: []string{"flyers", "philadelphia flyers"}},
	{League: espn.LeagueNHL, TeamQuery: "Minnesota Wild", Aliases: []string{"minnesota wild"}},
	{League: espn.LeagueNHL, TeamQuery: "Los Angeles Kings", Aliases: []string{"los angeles kings", "la kings"}},
	{League: LeagueIPL, TeamQuery: "Chennai Super Kings", Aliases: []string{"chennai super kings", "csk"}},
	{League: LeagueIPL, TeamQuery: "Mumbai Indians", Aliases: []string{"mumbai indians", "mi"}},
	{League: LeagueIPL, TeamQuery: "Royal Challengers Bengaluru", Aliases: []string{"royal challengers bengaluru", "royal challengers bangalore", "rcb"}},
	{League: LeagueIPL, TeamQuery: "Kolkata Knight Riders", Aliases: []string{"kolkata knight riders", "kkr"}},
	{League: LeagueIPL, TeamQuery: "Delhi Capitals", Aliases: []string{"delhi capitals", "dc"}},
	{League: LeagueIPL, TeamQuery: "Rajasthan Royals", Aliases: []string{"rajasthan royals", "rr"}},
	{League: LeagueIPL, TeamQuery: "Punjab Kings", Aliases: []string{"punjab kings", "pbks", "kings xi punjab", "kxip"}},
	{League: LeagueIPL, TeamQuery: "Sunrisers Hyderabad", Aliases: []string{"sunrisers hyderabad", "srh"}},
	{League: LeagueIPL, TeamQuery: "Gujarat Titans", Aliases: []string{"gujarat titans", "gt"}},
	{League: LeagueIPL, TeamQuery: "Lucknow Super Giants", Aliases: []string{"lucknow super giants", "lsg"}},
	// EPL clubs
	{League: espn.LeagueEPL, TeamQuery: "Arsenal", Aliases: []string{"arsenal", "the gunners"}},
	{League: espn.LeagueEPL, TeamQuery: "Chelsea", Aliases: []string{"chelsea", "the blues"}},
	{League: espn.LeagueEPL, TeamQuery: "Liverpool", Aliases: []string{"liverpool", "the reds"}},
	{League: espn.LeagueEPL, TeamQuery: "Manchester City", Aliases: []string{"manchester city", "man city", "man. city", "mcfc"}},
	{League: espn.LeagueEPL, TeamQuery: "Manchester United", Aliases: []string{"manchester united", "man united", "man utd", "mufc"}},
	{League: espn.LeagueEPL, TeamQuery: "Tottenham Hotspur", Aliases: []string{"tottenham", "tottenham hotspur", "spurs fc"}},
	{League: espn.LeagueEPL, TeamQuery: "Newcastle United", Aliases: []string{"newcastle", "newcastle united"}},
	{League: espn.LeagueEPL, TeamQuery: "Aston Villa", Aliases: []string{"aston villa", "villa"}},
	// La Liga clubs
	{League: espn.LeagueLaLiga, TeamQuery: "Real Madrid", Aliases: []string{"real madrid", "madrid"}},
	{League: espn.LeagueLaLiga, TeamQuery: "FC Barcelona", Aliases: []string{"barcelona", "fc barcelona", "barca"}},
	{League: espn.LeagueLaLiga, TeamQuery: "Atletico Madrid", Aliases: []string{"atletico madrid", "atletico", "atleti"}},
	// Bundesliga clubs
	{League: espn.LeagueBundesliga, TeamQuery: "Bayern Munich", Aliases: []string{"bayern munich", "fc bayern", "fc bayern munich", "bayern", "fcb"}},
	{League: espn.LeagueBundesliga, TeamQuery: "Borussia Dortmund", Aliases: []string{"borussia dortmund", "dortmund", "bvb"}},
	{League: espn.LeagueBundesliga, TeamQuery: "RB Leipzig", Aliases: []string{"rb leipzig", "leipzig", "rbl"}},
	{League: espn.LeagueBundesliga, TeamQuery: "Bayer Leverkusen", Aliases: []string{"bayer leverkusen", "leverkusen", "bayer 04"}},
	{League: espn.LeagueBundesliga, TeamQuery: "Borussia Monchengladbach", Aliases: []string{"borussia monchengladbach", "monchengladbach", "gladbach", "borus monchengladbach"}},
	{League: espn.LeagueBundesliga, TeamQuery: "Werder Bremen", Aliases: []string{"werder bremen", "bremen", "sv werder"}},
	{League: espn.LeagueBundesliga, TeamQuery: "Eintracht Frankfurt", Aliases: []string{"eintracht frankfurt", "frankfurt", "sge"}},
	{League: espn.LeagueBundesliga, TeamQuery: "VfB Stuttgart", Aliases: []string{"vfb stuttgart", "stuttgart"}},
	{League: espn.LeagueBundesliga, TeamQuery: "TSG Hoffenheim", Aliases: []string{"tsg hoffenheim", "hoffenheim", "tsg 1899"}},
	{League: espn.LeagueBundesliga, TeamQuery: "SC Freiburg", Aliases: []string{"sc freiburg", "freiburg"}},
	{League: espn.LeagueBundesliga, TeamQuery: "Union Berlin", Aliases: []string{"union berlin", "fc union berlin", "1 fc union berlin"}},
	{League: espn.LeagueBundesliga, TeamQuery: "VfL Wolfsburg", Aliases: []string{"vfl wolfsburg", "wolfsburg"}},
	{League: espn.LeagueBundesliga, TeamQuery: "1. FSV Mainz 05", Aliases: []string{"mainz 05", "mainz", "fsv mainz"}},
	{League: espn.LeagueBundesliga, TeamQuery: "FC Augsburg", Aliases: []string{"fc augsburg", "augsburg"}},
	{League: espn.LeagueBundesliga, TeamQuery: "VfL Bochum", Aliases: []string{"vfl bochum", "bochum"}},
	{League: espn.LeagueBundesliga, TeamQuery: "1. FC Koln", Aliases: []string{"1 fc koln", "fc cologne", "cologne", "koln"}},
	{League: espn.LeagueBundesliga, TeamQuery: "Hertha Berlin", Aliases: []string{"hertha berlin", "hertha", "hertha bsc"}},
	{League: espn.LeagueBundesliga, TeamQuery: "Hamburger SV", Aliases: []string{"hamburger sv", "hamburg", "hsv"}},
	// Serie A clubs
	{League: espn.LeagueSerieA, TeamQuery: "Juventus", Aliases: []string{"juventus", "juve", "la vecchia signora"}},
	{League: espn.LeagueSerieA, TeamQuery: "Inter Milan", Aliases: []string{"inter milan", "inter", "nerazzurri", "fc internazionale"}},
	{League: espn.LeagueSerieA, TeamQuery: "AC Milan", Aliases: []string{"ac milan", "milan", "rossoneri"}},
	{League: espn.LeagueSerieA, TeamQuery: "Napoli", Aliases: []string{"napoli", "naples", "ssc napoli"}},
	{League: espn.LeagueSerieA, TeamQuery: "AS Roma", Aliases: []string{"as roma", "roma", "giallorossi"}},
	{League: espn.LeagueSerieA, TeamQuery: "Lazio", Aliases: []string{"lazio", "ss lazio"}},
	{League: espn.LeagueSerieA, TeamQuery: "Atalanta", Aliases: []string{"atalanta", "la dea"}},
	{League: espn.LeagueSerieA, TeamQuery: "Fiorentina", Aliases: []string{"fiorentina", "viola"}},
	{League: espn.LeagueSerieA, TeamQuery: "Torino", Aliases: []string{"torino", "toro", "il toro"}},
	{League: espn.LeagueSerieA, TeamQuery: "Bologna", Aliases: []string{"bologna", "bologna fc"}},
	{League: espn.LeagueSerieA, TeamQuery: "Udinese", Aliases: []string{"udinese"}},
	{League: espn.LeagueSerieA, TeamQuery: "Sassuolo", Aliases: []string{"sassuolo"}},
	{League: espn.LeagueSerieA, TeamQuery: "Monza", Aliases: []string{"monza", "ac monza"}},
	{League: espn.LeagueSerieA, TeamQuery: "Genoa", Aliases: []string{"genoa", "genoa cfc"}},
	{League: espn.LeagueSerieA, TeamQuery: "Cagliari", Aliases: []string{"cagliari"}},
}

type statMetricConfig struct {
	Aliases       []string
	DefaultLeague string
	Category      string
	StatName      string
	Label         string
	Sort          string
	DisplayName   string
	Ascending     bool
}

var statMetricConfigs = []statMetricConfig{
	{Aliases: []string{"hr", "home run", "home runs", "homer", "homers"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "homeRuns", Label: "HR", Sort: "batting.homeRuns:desc", DisplayName: "Home Runs"},
	{Aliases: []string{"rbi", "rbis", "runs batted in"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "RBIs", Label: "RBI", Sort: "batting.RBIs:desc", DisplayName: "RBI"},
	{Aliases: []string{"batting average", "avg"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "avg", Label: "AVG", Sort: "batting.avg:desc", DisplayName: "Batting Average"},
	{Aliases: []string{"hits"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "hits", Label: "H", Sort: "batting.hits:desc", DisplayName: "Hits"},
	{Aliases: []string{"stolen bases", "steals"}, DefaultLeague: espn.LeagueMLB, Category: "batting", StatName: "stolenBases", Label: "SB", Sort: "batting.stolenBases:desc", DisplayName: "Stolen Bases"},
	{Aliases: []string{"era", "earned run average"}, DefaultLeague: espn.LeagueMLB, Category: "pitching", StatName: "ERA", Label: "ERA", Sort: "pitching.ERA:asc", DisplayName: "ERA", Ascending: true},
	{Aliases: []string{"strikeout", "strikeouts", "ks"}, DefaultLeague: espn.LeagueMLB, Category: "pitching", StatName: "strikeouts", Label: "K", Sort: "pitching.strikeouts:desc", DisplayName: "Strikeouts"},
	{Aliases: []string{"saves"}, DefaultLeague: espn.LeagueMLB, Category: "pitching", StatName: "saves", Label: "SV", Sort: "pitching.saves:desc", DisplayName: "Saves"},
	{Aliases: []string{"whip"}, DefaultLeague: espn.LeagueMLB, Category: "pitching", StatName: "WHIP", Label: "WHIP", Sort: "pitching.WHIP:asc", DisplayName: "WHIP", Ascending: true},
	{Aliases: []string{"passing yards"}, DefaultLeague: espn.LeagueNFL, Category: "passing", StatName: "passingYards", Label: "YDS", Sort: "passing.passingYards:desc", DisplayName: "Passing Yards"},
	{Aliases: []string{"passing touchdowns", "passing tds", "passing td"}, DefaultLeague: espn.LeagueNFL, Category: "passing", StatName: "passingTouchdowns", Label: "TD", Sort: "passing.passingTouchdowns:desc", DisplayName: "Passing Touchdowns"},
	{Aliases: []string{"rushing yards"}, DefaultLeague: espn.LeagueNFL, Category: "rushing", StatName: "rushingYards", Label: "YDS", Sort: "rushing.rushingYards:desc", DisplayName: "Rushing Yards"},
	{Aliases: []string{"rushing touchdowns", "rushing tds", "rushing td", "rushing tbs"}, DefaultLeague: espn.LeagueNFL, Category: "rushing", StatName: "rushingTouchdowns", Label: "TD", Sort: "rushing.rushingTouchdowns:desc", DisplayName: "Rushing Touchdowns"},
	{Aliases: []string{"receiving yards"}, DefaultLeague: espn.LeagueNFL, Category: "receiving", StatName: "receivingYards", Label: "YDS", Sort: "receiving.receivingYards:desc", DisplayName: "Receiving Yards"},
	{Aliases: []string{"receiving touchdowns", "receiving tds", "receiving td"}, DefaultLeague: espn.LeagueNFL, Category: "receiving", StatName: "receivingTouchdowns", Label: "TD", Sort: "receiving.receivingTouchdowns:desc", DisplayName: "Receiving Touchdowns"},
	{Aliases: []string{"receptions", "catches"}, DefaultLeague: espn.LeagueNFL, Category: "receiving", StatName: "receptions", Label: "REC", Sort: "receiving.receptions:desc", DisplayName: "Receptions"},
	{Aliases: []string{"interceptions", "picks", "pick sixes"}, DefaultLeague: espn.LeagueNFL, Category: "defensive", StatName: "interceptions", Label: "INT", Sort: "defensive.interceptions:desc", DisplayName: "Interceptions"},
	{Aliases: []string{"sacks"}, DefaultLeague: espn.LeagueNFL, Category: "defensive", StatName: "sacks", Label: "SACKS", Sort: "defensive.sacks:desc", DisplayName: "Sacks"},
	{Aliases: []string{"points per game", "ppg", "points"}, DefaultLeague: espn.LeagueNBA, Category: "offensive", StatName: "avgPoints", Label: "PTS", Sort: "offensive.avgPoints:desc", DisplayName: "Points Per Game"},
	{Aliases: []string{"rebounds", "rebounds per game", "rpg"}, DefaultLeague: espn.LeagueNBA, Category: "general", StatName: "avgRebounds", Label: "REB", Sort: "general.avgRebounds:desc", DisplayName: "Rebounds Per Game"},
	{Aliases: []string{"assists", "assists per game", "apg"}, DefaultLeague: espn.LeagueNBA, Category: "offensive", StatName: "avgAssists", Label: "AST", Sort: "offensive.avgAssists:desc", DisplayName: "Assists Per Game"},
	{Aliases: []string{"steals", "steals per game"}, DefaultLeague: espn.LeagueNBA, Category: "defensive", StatName: "avgSteals", Label: "STL", Sort: "defensive.avgSteals:desc", DisplayName: "Steals Per Game"},
	{Aliases: []string{"blocks", "blocks per game"}, DefaultLeague: espn.LeagueNBA, Category: "defensive", StatName: "avgBlocks", Label: "BLK", Sort: "defensive.avgBlocks:desc", DisplayName: "Blocks Per Game"},
	{Aliases: []string{"goals"}, DefaultLeague: espn.LeagueNHL, Category: "scoring", StatName: "goals", Label: "G", Sort: "scoring.goals:desc", DisplayName: "Goals"},
	{Aliases: []string{"hockey assists", "assists"}, DefaultLeague: espn.LeagueNHL, Category: "scoring", StatName: "assists", Label: "A", Sort: "scoring.assists:desc", DisplayName: "Assists"},
	{Aliases: []string{"hockey points", "points"}, DefaultLeague: espn.LeagueNHL, Category: "scoring", StatName: "points", Label: "PTS", Sort: "scoring.points:desc", DisplayName: "Points"},
}

func DetectSportsIntent(query string, now time.Time) (*SportsRequest, bool) {
	raw := strings.TrimSpace(query)
	if raw == "" {
		return nil, false
	}

	norm := normalizeText(raw)
	if isNonLookupQuery(norm) {
		return nil, false
	}

	cfg, hasLeague := detectLeague(norm)
	teamQuery := ""
	if team, ok := detectTeamAlias(norm); ok {
		teamQuery = team.TeamQuery
		if !hasLeague {
			if teamCfg, ok := leagueConfigByLeague(team.League); ok {
				cfg = teamCfg
				hasLeague = true
			}
		}
	}

	intent := detectIntent(norm)
	season := parseSeasonFromQuery(raw)
	limit := parseLimitFromQuery(norm, defaultLimitForIntent(intent))
	if intent == SportsIntentSchedule && teamQuery != "" && !hasTemporalPhrase(norm) {
		intent = SportsIntentTeamSchedule
	}
	if intent == SportsIntentTeamRecord && teamQuery == "" {
		return nil, false
	}
	if metric, ok := detectStatMetric(norm, cfg, hasLeague); ok && isLeaderQuery(norm) {
		intent = SportsIntentLeaders
		if !hasLeague && metric.DefaultLeague != "" {
			if metricCfg, ok := leagueConfigByLeague(metric.DefaultLeague); ok {
				cfg = metricCfg
				hasLeague = true
			}
		}
		if limit == defaultLimitForIntent(SportsIntentUnknown) || limit == 0 {
			limit = 50
		}
		return sportsRequestWithStat(raw, cfg, hasLeague, teamQuery, "", intent, metric, season, limit), hasLeague
	}
	if intent == SportsIntentAthleteStats || intent == SportsIntentAthleteNews {
		athleteQuery := extractAthleteQuery(raw, norm, cfg, hasLeague, intent)
		if intent == SportsIntentAthleteStats && hasLeague &&
			(athleteQuery == "" || isTeamStatsQuery(norm, athleteQuery, teamQuery)) {
			date, dateLabel, _ := parseDateFromQuery(raw, norm, now, SportsIntentLeagueStats)
			return &SportsRequest{
				RawQuery:  raw,
				Intent:    SportsIntentLeagueStats,
				League:    cfg.League,
				Sport:     cfg.Sport,
				TeamQuery: teamQuery,
				Date:      date,
				DateLabel: dateLabel,
				Season:    season,
				Limit:     limit,
			}, true
		}
		if athleteQuery == "" {
			return nil, false
		}
		req := sportsRequestWithStat(raw, cfg, hasLeague, teamQuery, athleteQuery, intent, statMetricConfig{}, season, limit)
		return req, true
	}

	// ── Extended-capability intents ─────────────────────────────────────────
	switch intent {
	case SportsIntentSearch:
		sq := extractSearchQuery(raw)
		return &SportsRequest{
			RawQuery:     raw,
			Intent:       SportsIntentSearch,
			League:       cfg.League,
			Sport:        cfg.Sport,
			AthleteQuery: sq,
			Season:       season,
			Limit:        limit,
		}, true

	case SportsIntentAthleteComparison:
		first, second := extractTwoAthletes(raw)
		if first == "" || second == "" {
			return nil, false
		}
		return &SportsRequest{
			RawQuery:           raw,
			Intent:             SportsIntentAthleteComparison,
			League:             cfg.League,
			Sport:              cfg.Sport,
			AthleteQuery:       first,
			SecondAthleteQuery: second,
			Season:             season,
			Limit:              limit,
		}, true

	case SportsIntentHotZones:
		athleteQuery := extractAthleteQuery(raw, norm, cfg, hasLeague, intent)
		if athleteQuery == "" {
			return nil, false
		}
		return &SportsRequest{
			RawQuery:     raw,
			Intent:       SportsIntentHotZones,
			League:       cfg.League,
			Sport:        cfg.Sport,
			AthleteQuery: athleteQuery,
			Season:       season,
			Limit:        limit,
		}, true

	case SportsIntentGameDetail:
		if !hasLeague {
			return nil, false
		}
		return &SportsRequest{
			RawQuery:          raw,
			Intent:            SportsIntentGameDetail,
			League:            cfg.League,
			Sport:             cfg.Sport,
			TeamQuery:         teamQuery,
			GameDetailSubtype: detectGameDetailSubtype(norm),
			Season:            season,
			Limit:             limit,
		}, true

	case SportsIntentQBR:
		if !hasLeague {
			// QBR is primarily NFL; default to it when no league is specified
			if qbrCfg, ok := leagueConfigByLeague(espn.LeagueNFL); ok {
				cfg = qbrCfg
				hasLeague = true
			}
		}
		return &SportsRequest{
			RawQuery:     raw,
			Intent:       SportsIntentQBR,
			League:       cfg.League,
			Sport:        cfg.Sport,
			AthleteQuery: extractAthleteQuery(raw, norm, cfg, hasLeague, intent),
			Season:       season,
			Limit:        limit,
		}, hasLeague

	case SportsIntentChampions:
		if !hasLeague {
			if champCfg, ok := detectLeagueFromChampionship(norm); ok {
				cfg = champCfg
				hasLeague = true
			}
		}
		return &SportsRequest{
			RawQuery:  raw,
			Intent:    SportsIntentChampions,
			League:    cfg.League,
			Sport:     cfg.Sport,
			TeamQuery: teamQuery,
			Season:    season,
			Limit:     limit,
		}, hasLeague

	case SportsIntentDraft:
		if !hasLeague {
			return nil, false
		}
		return &SportsRequest{
			RawQuery:  raw,
			Intent:    SportsIntentDraft,
			League:    cfg.League,
			Sport:     cfg.Sport,
			TeamQuery: teamQuery,
			Season:    season,
			Limit:     limit,
		}, true

	case SportsIntentCoaches:
		if !hasLeague {
			return nil, false
		}
		return &SportsRequest{
			RawQuery:  raw,
			Intent:    SportsIntentCoaches,
			League:    cfg.League,
			Sport:     cfg.Sport,
			TeamQuery: teamQuery,
			Season:    season,
			Limit:     limit,
		}, true

	case SportsIntentVenues:
		if !hasLeague {
			return nil, false
		}
		return &SportsRequest{
			RawQuery:  raw,
			Intent:    SportsIntentVenues,
			League:    cfg.League,
			Sport:     cfg.Sport,
			TeamQuery: teamQuery,
			Season:    season,
			Limit:     limit,
		}, true

	case SportsIntentPowerIndex:
		// Default to CFB when no league specified (FPI is primarily CFB/CBB)
		if !hasLeague {
			if piCfg, ok := leagueConfigByLeague(espn.LeagueCollegeFootball); ok {
				cfg = piCfg
				hasLeague = true
			}
		}
		return &SportsRequest{
			RawQuery:  raw,
			Intent:    SportsIntentPowerIndex,
			League:    cfg.League,
			Sport:     cfg.Sport,
			TeamQuery: teamQuery,
			Season:    season,
			Limit:     limit,
		}, hasLeague

	case SportsIntentRecruits:
		// Default to CFB when no league specified
		if !hasLeague {
			if rCfg, ok := leagueConfigByLeague(espn.LeagueCollegeFootball); ok {
				cfg = rCfg
				hasLeague = true
			}
		}
		return &SportsRequest{
			RawQuery:  raw,
			Intent:    SportsIntentRecruits,
			League:    cfg.League,
			Sport:     cfg.Sport,
			TeamQuery: teamQuery,
			Season:    season,
			Limit:     limit,
		}, hasLeague

	case SportsIntentBracketology:
		return &SportsRequest{
			RawQuery: raw,
			Intent:   SportsIntentBracketology,
			League:   cfg.League,
			Sport:    cfg.Sport,
			Season:   season,
			Limit:    limit,
		}, true
	}

	if intent == SportsIntentUnknown {
		if !hasLeague {
			return nil, false
		}
		if hasTemporalPhrase(norm) {
			intent = SportsIntentScores
			if hasAnyPhrase(norm, "playing", "play", "who plays", "games", "game", "match", "matches", "matchup", "matchups") {
				intent = SportsIntentSchedule
			}
		} else {
			return nil, false
		}
	}

	if !hasLeague && intent != SportsIntentNews && intent != SportsIntentOdds {
		return nil, false
	}
	if !hasLeague && intent == SportsIntentNews && !hasBroadSportsNewsPhrase(norm) {
		// Fall back to athlete news when an athlete name can be extracted from the
		// query.  Guard against generic phrases like "latest sports movies" whose
		// cleaned residual still contains the word "sports".
		if aq := extractAthleteQuery(raw, norm, LeagueConfig{}, false, SportsIntentAthleteNews); aq != "" && !hasPhrase(aq, "sports") {
			return &SportsRequest{
				RawQuery:     raw,
				Intent:       SportsIntentAthleteNews,
				AthleteQuery: aq,
				Season:       season,
				Limit:        defaultLimitForIntent(SportsIntentNews),
			}, true
		}
		return nil, false
	}
	if !hasLeague && intent == SportsIntentOdds && !hasBroadSportsOddsPhrase(norm) {
		return nil, false
	}

	date, dateLabel, _ := parseDateFromQuery(raw, norm, now, intent)
	return &SportsRequest{
		RawQuery:  raw,
		Intent:    intent,
		League:    cfg.League,
		Sport:     cfg.Sport,
		TeamQuery: teamQuery,
		Date:      date,
		DateLabel: dateLabel,
		Season:    season,
		Limit:     limit,
	}, true
}

func ParseDateValue(value string, now time.Time, intent SportsIntentType) (*time.Time, string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, "", nil
	}
	norm := normalizeText(raw)
	date, label, ok := parseDateFromQuery(raw, norm, now, intent)
	if ok {
		return date, label, nil
	}

	loc := now.Location()
	if loc == nil {
		loc = time.Local
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		return datePtr(t), t.Format("Jan 2, 2006"), nil
	}
	if t, err := time.ParseInLocation("1/2/2006", raw, loc); err == nil {
		return datePtr(t), t.Format("Jan 2, 2006"), nil
	}
	return nil, "", ErrMalformedDate
}

func ValidateDateInQuery(query string, now time.Time) error {
	loc := now.Location()
	if loc == nil {
		loc = time.Local
	}
	if match := exactDatePattern.FindString(query); match != "" {
		if _, err := time.ParseInLocation("2006-01-02", match, loc); err != nil {
			return ErrMalformedDate
		}
	}
	if match := slashDatePattern.FindString(query); match != "" {
		if _, err := time.ParseInLocation("1/2/2006", match, loc); err != nil {
			return ErrMalformedDate
		}
	}
	return nil
}

func normalizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func hasPhrase(norm, phrase string) bool {
	needle := normalizeText(phrase)
	if needle == "" {
		return false
	}
	return strings.Contains(" "+norm+" ", " "+needle+" ")
}

func hasAnyPhrase(norm string, phrases ...string) bool {
	for _, phrase := range phrases {
		if hasPhrase(norm, phrase) {
			return true
		}
	}
	return false
}

func isNonLookupQuery(norm string) bool {
	if hasAnyPhrase(norm,
		"write a story", "short story", "make a sports logo", "sports logo",
		"write a sports news article", "write a news article", "draft a sports news article",
	) {
		return true
	}
	if hasAnyPhrase(norm, "explain betting odds", "explain how betting odds work", "how betting odds work", "how do betting odds work") {
		return true
	}
	if hasPhrase(norm, "explain how") && hasAnyPhrase(norm, "standings work", "works", "work", "calculated", "determined") {
		return true
	}
	if hasPhrase(norm, "how standings work") || hasPhrase(norm, "how mlb standings work") {
		return true
	}
	if hasPhrase(norm, "is calculated") || hasPhrase(norm, "are calculated") {
		return true
	}
	// Allow historical champion queries through before blocking on history/all-time keywords.
	if !hasAnyPhrase(norm, "champion", "championship", "won the", "winner", "super bowl", "stanley cup", "world series") {
		if hasAnyPhrase(norm, "in history", "all time record", "history", "all time") {
			return true
		}
	}
	return false
}

func detectLeague(norm string) (LeagueConfig, bool) {
	var best LeagueConfig
	bestLen := 0
	for _, cfg := range leagueConfigs {
		for _, alias := range cfg.Aliases {
			if hasPhrase(norm, alias) {
				if l := len(normalizeText(alias)); l > bestLen {
					best = cfg
					bestLen = l
				}
			}
		}
	}
	if bestLen > 0 {
		return best, true
	}
	return LeagueConfig{}, false
}

func detectTeamAlias(norm string) (teamAlias, bool) {
	for _, team := range teamAliases {
		for _, alias := range team.Aliases {
			if hasPhrase(norm, alias) {
				return team, true
			}
		}
	}
	// Fuzzy fallback: single-token typo tolerance (edit distance ≤ 1)
	return detectTeamAliasFuzzy(norm)
}

// levenshteinDistance computes the minimum edit distance between two strings.
func levenshteinDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			best := prev[j] + 1
			if curr[j-1]+1 < best {
				best = curr[j-1] + 1
			}
			if prev[j-1]+cost < best {
				best = prev[j-1] + cost
			}
			curr[j] = best
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// detectTeamAliasFuzzy tries single-word tokens in norm against single-word aliases
// using Levenshtein distance ≤ 1 for tokens of length ≥ 4, providing typo tolerance.
func detectTeamAliasFuzzy(norm string) (teamAlias, bool) {
	tokens := strings.Fields(norm)
	for _, team := range teamAliases {
		for _, alias := range team.Aliases {
			aliasToks := strings.Fields(alias)
			if len(aliasToks) != 1 {
				continue // fuzzy only on single-word aliases to avoid false positives
			}
			aliasWord := aliasToks[0]
			if len(aliasWord) < 6 {
				continue // require 6+ chars to avoid false positives (e.g. "news"→"nets", "stats"→"stars")
			}
			for _, tok := range tokens {
				if len(tok) < 6 {
					continue
				}
				if levenshteinDistance(tok, aliasWord) == 1 {
					return team, true
				}
			}
		}
	}
	return teamAlias{}, false
}

func detectIntent(norm string) SportsIntentType {
	// ── Extended capabilities: checked first to avoid overlap ──────────────

	// Bracketology (Q87): NCAA bracket projections — before generic schedule/standings
	if hasAnyPhrase(norm, "bracketology", "bracket projection", "bracket projections",
		"bracket outlook", "ncaa bracket", "tournament bracket", "march madness bracket") {
		return SportsIntentBracketology
	}
	// Recruiting / recruiting class (Q85-Q86): CFB recruits
	if hasAnyPhrase(norm, "recruiting class", "top recruits", "top prospects", "top commits",
		"recruit rankings", "recruiting rankings") ||
		(hasAnyPhrase(norm, "recruit", "recruits") && hasAnyPhrase(norm, "rankings", "rank", "top", "class")) {
		return SportsIntentRecruits
	}
	// Power Index / FPI / BPI / SP+ (Q83-Q84): must come before generic rankings
	if hasAnyPhrase(norm, "power index", "fpi", "bpi", "sp+", "strength of schedule power") ||
		(hasAnyPhrase(norm, "power index", "power ranking") && hasAnyPhrase(norm, "cfb", "college football", "college basketball")) {
		return SportsIntentPowerIndex
	}
	// Venues / stadiums / arenas (Q77-Q78)
	if hasAnyPhrase(norm, "home stadium", "home arena", "home ballpark", "home venue", "home venues", "home field",
		"what stadium", "which stadium", "what arena", "which arena", "what ballpark", "which ballpark") {
		return SportsIntentVenues
	}

	// Champions history (Q69-Q72): must come before scores ("who won")
	if hasAnyPhrase(norm,
		"super bowl winner", "super bowl champion",
		"nba champion", "nba finals winner", "nba championship winner",
		"stanley cup winner", "stanley cup champion",
		"world series winner", "world series champion",
		"championship history", "champions history",
		"who won the super bowl", "who won the stanley cup",
		"who won the world series", "who won the nba championship", "who won the nba finals",
		"last champion", "recent champion", "past champion",
	) {
		return SportsIntentChampions
	}
	// Champions with a year: "who won the 2024 Super Bowl", "2023 World Series winner"
	if hasAnyPhrase(norm, "super bowl", "world series", "stanley cup", "nba finals", "nba championship") &&
		hasAnyPhrase(norm, "won", "winner", "champion", "champions", "championship") {
		return SportsIntentChampions
	}
	// Draft (Q73-Q74): before schedule/transactions
	if hasAnyPhrase(norm, "draft results", "draft picks", "draft class", "nfl draft", "nba draft", "mlb draft", "nhl draft", "cfb draft") ||
		(hasPhrase(norm, "draft") && hasAnyPhrase(norm, "round", "pick", "overall", "selection")) {
		return SportsIntentDraft
	}
	// Coaches (Q75-Q76): before roster (coaching staff ≠ player roster)
	if hasAnyPhrase(norm, "head coach", "head coaches", "coaching staff", "who is the coach", "who coaches") ||
		(hasPhrase(norm, "coaches") && !hasPhrase(norm, "poll")) {
		return SportsIntentCoaches
	}
	// Game detail subtypes (Q58 win prob / Q62 predictor / Q63 officials / Q68 CDN package)
	if hasAnyPhrase(norm, "who are the officials", "who are the refs", "who are the referees",
		"game officials", "assigned officials", "officiating crew") {
		return SportsIntentGameDetail
	}
	if hasAnyPhrase(norm, "win probability", "win prob", "winning probability") {
		return SportsIntentGameDetail
	}
	if hasAnyPhrase(norm, "game predictor", "espn predictor", "game prediction", "predictor for") {
		return SportsIntentGameDetail
	}
	if hasAnyPhrase(norm, "game package", "full game package") {
		return SportsIntentGameDetail
	}
	// QBR (Q46)
	if hasAnyPhrase(norm, "qbr", "quarterback rating", "total qbr") {
		return SportsIntentQBR
	}
	// ESPN search (Q10)
	if hasAnyPhrase(norm, "search espn for", "search espn", "espn search", "find on espn") {
		return SportsIntentSearch
	}
	// Hot zones (Q53): athlete-level, detected before generic AthleteStats
	if hasAnyPhrase(norm, "hot zone", "hot zones") {
		return SportsIntentHotZones
	}
	// Athlete comparison (Q52): "compare X and Y" or "X vs Y"
	if hasAnyPhrase(norm, "compare", "head to head") && !hasOddsIntent(norm) {
		return SportsIntentAthleteComparison
	}

	// ── Existing intent detection ───────────────────────────────────────────
	if hasAnyPhrase(norm, "roster", "depth chart", "who is on", "who plays for") {
		return SportsIntentRoster
	}
	if hasAnyPhrase(norm, "injury", "injuries", "injured", "injury report") {
		return SportsIntentInjuries
	}
	if hasAnyPhrase(norm, "transaction", "transactions", "traded", "signed", "waived") {
		return SportsIntentTransactions
	}
	if hasAnyPhrase(norm, "team record", "record") && !hasAnyPhrase(norm, "record holder", "records", "all time") {
		return SportsIntentTeamRecord
	}
	if hasAnyPhrase(norm, "team schedule", "full schedule", "season schedule") {
		return SportsIntentTeamSchedule
	}
	if hasAnyPhrase(norm, "player news", "athlete news") {
		return SportsIntentAthleteNews
	}
	if hasOddsIntent(norm) {
		return SportsIntentOdds
	}
	if isLeaderQuery(norm) {
		return SportsIntentLeaders
	}
	if hasPlayerStatMetric(norm) {
		return SportsIntentAthleteStats
	}
	if hasAnyPhrase(norm, "player stats", "player statistics", "athlete stats", "athlete statistics", "stats", "statistics", "game log", "gamelog", "splits", "bio") &&
		!hasAnyPhrase(norm, "team stats", "team statistics", "league stats", "league statistics") {
		return SportsIntentAthleteStats
	}
	if hasAnyPhrase(norm, "team stats", "team statistics", "league stats", "league statistics") {
		return SportsIntentLeagueStats
	}
	if hasAnyPhrase(norm, "top 25", "poll", "polls", "ap poll", "coaches poll") ||
		(hasPhrase(norm, "rankings") && hasAnyPhrase(norm,
			"college football", "ncaaf", "cfb", "college basketball", "ncaamb", "ncaawb",
		)) {
		return SportsIntentRankings
	}
	if hasAnyPhrase(norm,
		"standings", "conference standings", "division", "rank", "rankings",
		"wild card", "wildcard",
	) || hasStandingsTableIntent(norm) {
		return SportsIntentStandings
	}
	if hasAnyPhrase(norm,
		"score", "scores", "final", "result", "results", "how did", "who won", "live score",
	) {
		return SportsIntentScores
	}
	if hasAnyPhrase(norm,
		"schedule", "games", "game", "match", "matches", "next game", "next match",
		"playing", "who plays", "matchup", "matchups",
		"on today", "tonight", "tomorrow",
	) {
		return SportsIntentSchedule
	}
	if hasAnyPhrase(norm,
		"news", "headlines", "latest", "latest on", "what is new", "what s new",
	) {
		return SportsIntentNews
	}
	return SportsIntentUnknown
}

func hasTemporalPhrase(norm string) bool {
	return hasAnyPhrase(norm, "today", "tonight", "current", "yesterday", "tomorrow", "this week", "this weekend") ||
		exactDatePattern.MatchString(norm) ||
		slashDatePattern.MatchString(norm)
}

func hasStandingsTableIntent(norm string) bool {
	if !hasPhrase(norm, "table") {
		return false
	}
	return hasAnyPhrase(norm,
		"league table",
		"premier league table",
		"english premier league table",
		"epl table",
		"soccer table",
		"points table",
		"ipl table",
		"ipl points table",
		"indian premier league table",
		"indian premier league points table",
		"indian premier cricket league table",
	) || hasAnyPhrase(norm,
		"premier league",
		"english premier league",
		"epl",
		"ipl",
		"indian premier league",
		"indian premier cricket league",
	)
}

func hasBroadSportsNewsPhrase(norm string) bool {
	if hasPhrase(norm, "sports") &&
		hasAnyPhrase(norm,
			"news", "headlines", "sports news", "sports headlines",
			"latest in sports", "what is new in sports", "what s new in sports",
		) {
		return true
	}
	return hasPhrase(norm, "espn") &&
		hasAnyPhrase(norm,
			"news", "headlines", "latest news", "latest espn news", "espn news", "espn headlines",
			"what is new", "what s new",
		)
}

func hasBroadSportsOddsPhrase(norm string) bool {
	return hasAnyPhrase(norm,
		"sports betting odds", "sports odds", "current betting odds",
		"latest betting odds", "today betting odds", "today s betting odds", "todays betting odds",
		"current betting lines", "latest betting lines", "sports betting lines",
	)
}

func hasOddsIntent(norm string) bool {
	return hasAnyPhrase(norm,
		"odds", "betting odds", "sportsbook odds", "betting lines", "game lines",
		"spread", "spreads", "point spread", "moneyline", "money line",
		"over under", "overunder", "over odds", "under odds",
		"who is favored", "who s favored", "who is the favorite", "point total",
	)
}

func defaultLimitForIntent(intent SportsIntentType) int {
	if intent == SportsIntentNews {
		return 10
	}
	if intent == SportsIntentLeaders {
		return 50
	}
	if intent == SportsIntentOdds {
		return 50
	}
	return 100
}

func isUnsupportedSportsStatQuery(norm string) bool {
	return false
}

func isTeamStatsQuery(norm, athleteQuery, teamQuery string) bool {
	if teamQuery == "" {
		return false
	}
	if hasAnyPhrase(norm, "team stats", "team statistics") {
		return true
	}
	team, ok := detectTeamAlias(normalizeText(athleteQuery))
	return ok && strings.EqualFold(team.TeamQuery, teamQuery)
}

func hasPlayerStatMetric(norm string) bool {
	return hasAnyPhrase(norm,
		"hr", "home run", "home runs", "homer", "homers",
		"rbi", "rbis", "runs batted in", "batting average", "avg", "hits", "stolen bases",
		"era", "earned run average", "strikeout", "strikeouts", "saves", "whip",
		"passing yards", "rushing yards", "receiving yards",
		"passing touchdowns", "passing tds", "rushing touchdowns", "rushing tds",
		"receiving touchdowns", "receiving tds", "receptions", "catches",
		"interceptions", "picks", "sacks",
		"touchdown", "touchdowns", "td", "tds",
		"points per game", "ppg", "rebounds", "assists", "steals", "blocks",
		"goals", "goal scorers", "clean sheets", "saves percentage",
	)
}

func isLeaderQuery(norm string) bool {
	return hasAnyPhrase(norm,
		"leader", "leaders", "leaderboard", "stat leaders", "league leaders",
		"top", "top players", "most", "who leads", "who led", "led the", "league leader",
		"highest", "lowest", "best", "fewest",
	)
}

func detectStatMetric(norm string, cfg LeagueConfig, hasLeague bool) (statMetricConfig, bool) {
	var fallback statMetricConfig
	for _, metric := range statMetricConfigs {
		for _, alias := range metric.Aliases {
			if !hasPhrase(norm, alias) {
				continue
			}
			if hasLeague && metric.DefaultLeague == cfg.League {
				return metric, true
			}
			if hasLeague {
				continue
			}
			if fallback.DisplayName == "" {
				fallback = metric
			}
		}
	}
	if fallback.DisplayName != "" {
		return fallback, true
	}
	return statMetricConfig{}, false
}

func sportsRequestWithStat(raw string, cfg LeagueConfig, hasLeague bool, teamQuery, athleteQuery string, intent SportsIntentType, metric statMetricConfig, season, limit int) *SportsRequest {
	req := &SportsRequest{
		RawQuery:     raw,
		Intent:       intent,
		TeamQuery:    teamQuery,
		AthleteQuery: athleteQuery,
		StatCategory: metric.Category,
		StatName:     metric.StatName,
		StatLabel:    metric.Label,
		StatSort:     metric.Sort,
		Season:       season,
		Limit:        limit,
	}
	if hasLeague {
		req.League = cfg.League
		req.Sport = cfg.Sport
	}
	return req
}

func parseSeasonFromQuery(raw string) int {
	for _, match := range seasonPattern.FindAllString(raw, -1) {
		if len(match) == 4 {
			if strings.Contains(raw, match+"-") || strings.Contains(raw, match+"/") {
				continue
			}
			var season int
			for _, r := range match {
				season = season*10 + int(r-'0')
			}
			return season
		}
	}
	return 0
}

func parseLimitFromQuery(norm string, fallback int) int {
	if fallback <= 0 {
		fallback = defaultLimitForIntent(SportsIntentUnknown)
	}
	match := topLimitPattern.FindStringSubmatch(norm)
	if len(match) == 2 {
		n := 0
		for _, r := range match[1] {
			n = n*10 + int(r-'0')
		}
		if n > 0 {
			if n > 100 {
				return 100
			}
			return n
		}
	}
	return fallback
}

func extractAthleteQuery(raw, norm string, cfg LeagueConfig, hasLeague bool, intent SportsIntentType) string {
	cleaned := normalizeText(raw)
	for _, phrase := range []string{
		"show me", "what are", "what is", "whats", "what s", "give me", "print out", "table",
		"player stats", "player statistics", "athlete stats", "athlete statistics",
		"stats", "statistics", "stat", "game log", "gamelog", "splits", "bio", "news",
		"for", "in", "during", "season", "regular season", "latest", "headlines",
	} {
		cleaned = removePhrase(cleaned, phrase)
	}
	if hasLeague {
		for _, alias := range cfg.Aliases {
			cleaned = removePhrase(cleaned, alias)
		}
		cleaned = removePhrase(cleaned, cfg.League)
	}
	for _, metric := range statMetricConfigs {
		for _, alias := range metric.Aliases {
			cleaned = removePhrase(cleaned, alias)
		}
	}
	cleaned = seasonPattern.ReplaceAllString(cleaned, " ")
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" && intent == SportsIntentAthleteNews {
		return strings.TrimSpace(raw)
	}
	return cleaned
}

func removePhrase(norm, phrase string) string {
	needle := normalizeText(phrase)
	if needle == "" {
		return norm
	}
	return strings.TrimSpace(strings.ReplaceAll(" "+norm+" ", " "+needle+" ", " "))
}

func parseDateFromQuery(raw, norm string, now time.Time, intent SportsIntentType) (*time.Time, string, bool) {
	loc := now.Location()
	if loc == nil {
		loc = time.Local
	}
	today := dateOnly(now.In(loc))

	// "last night" resolves to yesterday (more specific; checked before "yesterday").
	if hasPhrase(norm, "last night") {
		t := today.AddDate(0, 0, -1)
		return &t, "Yesterday", true
	}
	if hasPhrase(norm, "yesterday") {
		t := today.AddDate(0, 0, -1)
		return &t, "Yesterday", true
	}
	if hasPhrase(norm, "tomorrow") {
		t := today.AddDate(0, 0, 1)
		return &t, "Tomorrow", true
	}
	if hasPhrase(norm, "today") {
		return &today, "Today", true
	}
	if hasPhrase(norm, "tonight") {
		return &today, "Tonight", true
	}
	if hasPhrase(norm, "current") {
		if intent == SportsIntentStandings {
			return nil, "Current", true
		}
		return &today, "Today", true
	}

	// "this weekend" → nearest upcoming Saturday.
	if hasPhrase(norm, "this weekend") {
		daysUntilSat := int((time.Saturday - now.Weekday() + 7) % 7)
		if daysUntilSat == 0 {
			daysUntilSat = 7
		}
		t := today.AddDate(0, 0, daysUntilSat)
		return &t, "This Weekend", true
	}

	// "this week" → label only; no single-day anchor.
	if hasPhrase(norm, "this week") {
		return nil, "This Week", true
	}

	// Named holidays — next upcoming occurrence relative to today.
	// Order matters: check more-specific phrases before general ones.
	if hasPhrase(norm, "christmas day") || hasPhrase(norm, "christmas") {
		t := nextAnnualDate(today, time.December, 25)
		return &t, "Christmas Day", true
	}
	if hasPhrase(norm, "thanksgiving") {
		t := nextThanksgiving(today, loc)
		return &t, "Thanksgiving", true
	}
	if hasAnyPhrase(norm, "new year s eve", "new years eve") {
		t := nextAnnualDate(today, time.December, 31)
		return &t, "New Year's Eve", true
	}
	if hasAnyPhrase(norm, "new year s day", "new years day") {
		t := nextNewYearsDay(today, loc)
		return &t, "New Year's Day", true
	}
	if hasPhrase(norm, "new year") {
		t := nextNewYearsDay(today, loc)
		return &t, "New Year's Day", true
	}
	if hasAnyPhrase(norm, "independence day", "fourth of july", "july 4th", "july 4") {
		t := nextAnnualDate(today, time.July, 4)
		return &t, "Independence Day", true
	}
	if hasPhrase(norm, "super bowl sunday") {
		t := nextSuperBowlSunday(today, loc)
		return &t, "Super Bowl Sunday", true
	}

	if match := exactDatePattern.FindString(raw); match != "" {
		if t, err := time.ParseInLocation("2006-01-02", match, loc); err == nil {
			return &t, t.Format("Jan 2, 2006"), true
		}
	}
	if match := slashDatePattern.FindString(raw); match != "" {
		if t, err := time.ParseInLocation("1/2/2006", match, loc); err == nil {
			return &t, t.Format("Jan 2, 2006"), true
		}
	}

	// Named weekday (e.g. "Monday night", "Friday game") → next occurrence of
	// that weekday. Checked last to avoid shadowing the specific phrases above.
	weekdayNames := [7]struct {
		name string
		wd   time.Weekday
	}{
		{"sunday", time.Sunday}, {"monday", time.Monday}, {"tuesday", time.Tuesday},
		{"wednesday", time.Wednesday}, {"thursday", time.Thursday},
		{"friday", time.Friday}, {"saturday", time.Saturday},
	}
	for _, entry := range weekdayNames {
		if hasPhrase(norm, entry.name) {
			daysUntil := int((entry.wd - now.Weekday() + 7) % 7)
			if daysUntil == 0 {
				daysUntil = 7
			}
			t := today.AddDate(0, 0, daysUntil)
			return &t, t.Format("Mon Jan 2"), true
		}
	}

	return nil, "", false
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func datePtr(t time.Time) *time.Time {
	return &t
}

// nextAnnualDate returns the next occurrence of a fixed annual date (month,
// day) at or after today. If today equals the holiday, today is returned.
func nextAnnualDate(today time.Time, month time.Month, day int) time.Time {
	year := today.Year()
	t := time.Date(year, month, day, 0, 0, 0, 0, today.Location())
	if t.Before(today) {
		t = time.Date(year+1, month, day, 0, 0, 0, 0, today.Location())
	}
	return t
}

// nthWeekdayOfMonth returns the nth occurrence of a weekday in the given
// year/month (n is 1-based).
func nthWeekdayOfMonth(year int, month time.Month, wd time.Weekday, n int, loc *time.Location) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	daysUntil := int((wd - first.Weekday() + 7) % 7)
	return first.AddDate(0, 0, daysUntil+(n-1)*7)
}

// nextThanksgiving returns the 4th Thursday of November at or after today.
func nextThanksgiving(today time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	year := today.Year()
	t := nthWeekdayOfMonth(year, time.November, time.Thursday, 4, loc)
	if t.Before(today) {
		t = nthWeekdayOfMonth(year+1, time.November, time.Thursday, 4, loc)
	}
	return t
}

// nextNewYearsDay returns January 1st of the next year (or current year if
// today is Jan 1).
func nextNewYearsDay(today time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	year := today.Year()
	t := time.Date(year, time.January, 1, 0, 0, 0, 0, loc)
	if t.Before(today) {
		t = time.Date(year+1, time.January, 1, 0, 0, 0, 0, loc)
	}
	return t
}

// nextSuperBowlSunday returns the 2nd Sunday of February at or after today,
// which approximates when the NFL Super Bowl is typically played.
func nextSuperBowlSunday(today time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	year := today.Year()
	t := nthWeekdayOfMonth(year, time.February, time.Sunday, 2, loc)
	if t.Before(today) {
		t = nthWeekdayOfMonth(year+1, time.February, time.Sunday, 2, loc)
	}
	return t
}

func leagueConfigForRequest(req SportsRequest) (LeagueConfig, bool) {
	if req.League != "" {
		if cfg, ok := leagueConfigByLeague(req.League); ok {
			return cfg, true
		}
		if cfg, ok := leagueConfigByAlias(req.League); ok {
			return cfg, true
		}
	}
	if req.Sport != "" {
		for _, cfg := range leagueConfigs {
			if cfg.Sport == req.Sport && (req.League == "" || cfg.League == req.League) {
				return cfg, true
			}
		}
	}
	return LeagueConfig{}, false
}

func leagueConfigByLeague(league string) (LeagueConfig, bool) {
	for _, cfg := range leagueConfigs {
		if strings.EqualFold(cfg.League, strings.TrimSpace(league)) {
			return cfg, true
		}
	}
	return LeagueConfig{}, false
}

func leagueConfigByAlias(alias string) (LeagueConfig, bool) {
	norm := normalizeText(alias)
	for _, cfg := range leagueConfigs {
		if hasPhrase(norm, cfg.League) {
			return cfg, true
		}
		for _, a := range cfg.Aliases {
			if normalizeText(a) == norm {
				return cfg, true
			}
		}
	}
	return LeagueConfig{}, false
}

// ─── Extended-capability helpers ─────────────────────────────────────────────

var vsAthletePattern = regexp.MustCompile(`(?i)\s+(?:vs\.?|versus|and)\s+`)

// extractTwoAthletes parses "compare A and B" / "A vs B" patterns and returns
// the two athlete name fragments, or ("","") if the split cannot be determined.
func extractTwoAthletes(raw string) (string, string) {
	lower := strings.ToLower(raw)
	rest := raw
	for _, trigger := range []string{
		"compare ", "comparing ", "comparison between ",
		"head-to-head between ", "head to head between ",
		"head-to-head: ", "head to head: ",
		"head-to-head ", "head to head ",
	} {
		if idx := strings.Index(lower, trigger); idx >= 0 {
			rest = strings.TrimSpace(raw[idx+len(trigger):])
			break
		}
	}
	parts := vsAthletePattern.Split(rest, 2)
	if len(parts) < 2 {
		return "", ""
	}
	first := cleanAthleteToken(parts[0])
	second := cleanAthleteToken(parts[1])
	if first == "" || second == "" {
		return "", ""
	}
	// Reject if either part is a league/sport/stop-word rather than an athlete name
	if isLeagueOrStopWordToken(first) || isLeagueOrStopWordToken(second) {
		return "", ""
	}
	return first, second
}

// knownNonAthleteWords are single tokens that can never be part of an athlete name.
var knownNonAthleteWords = map[string]bool{
	// stop words
	"the": true, "a": true, "an": true, "this": true, "that": true,
	// sports
	"football": true, "basketball": true, "baseball": true, "hockey": true, "soccer": true,
	"sport": true, "sports": true, "game": true, "games": true, "match": true,
	// league abbreviations
	"nfl": true, "nba": true, "mlb": true, "nhl": true, "mls": true,
	"ncaa": true, "cfb": true, "wnba": true,
	// temporal / structural
	"offseason": true, "season": true, "league": true, "conference": true, "division": true,
}

// isLeagueOrStopWordToken returns true when every word in s is a known
// non-athlete token (league abbrev, sport name, stop word, etc.).
func isLeagueOrStopWordToken(s string) bool {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(s)))
	if len(words) == 0 {
		return true
	}
	for _, w := range words {
		if !knownNonAthleteWords[w] {
			return false
		}
	}
	return true
}

// cleanAthleteToken trims noise from an athlete name fragment extracted by
// extractTwoAthletes.
func cleanAthleteToken(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	for _, suffix := range []string{
		" in the nba", " in the nfl", " in the nhl", " in the mlb", " in the wnba",
		" stats", " statistics", " career", " season",
		" nba", " nfl", " nhl", " mlb", " wnba",
	} {
		if strings.HasSuffix(lower, suffix) {
			s = strings.TrimSpace(s[:len(s)-len(suffix)])
			lower = strings.ToLower(s)
		}
	}
	return s
}

// extractSearchQuery strips "search espn for …" / "espn search …" prefixes and
// returns the query term.
func extractSearchQuery(raw string) string {
	lower := strings.ToLower(raw)
	for _, prefix := range []string{
		"search espn for ", "search espn ", "espn search for ", "espn search ",
		"find on espn ", "look up on espn ",
	} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			return strings.TrimSpace(raw[idx+len(prefix):])
		}
	}
	return raw
}

// detectGameDetailSubtype returns the sub-operation string for a
// SportsIntentGameDetail request.
func detectGameDetailSubtype(norm string) string {
	if hasAnyPhrase(norm, "who are the officials", "who are the refs", "who are the referees",
		"game officials", "assigned officials", "officiating crew") {
		return "officials"
	}
	if hasAnyPhrase(norm, "win probability", "win prob", "winning probability") {
		return "probabilities"
	}
	if hasAnyPhrase(norm, "game predictor", "espn predictor", "game prediction", "predictor for") {
		return "predictor"
	}
	if hasAnyPhrase(norm, "game package", "full game package") {
		return "gamepackage"
	}
	return "summary"
}

// detectLeagueFromChampionship infers the league from championship-specific
// terminology (e.g. "Super Bowl" → NFL).
func detectLeagueFromChampionship(norm string) (LeagueConfig, bool) {
	if hasAnyPhrase(norm, "super bowl", "nfl champion", "nfl championship") {
		return leagueConfigByLeague(espn.LeagueNFL)
	}
	if hasAnyPhrase(norm, "nba champion", "nba finals", "nba championship", "basketball champion") {
		return leagueConfigByLeague(espn.LeagueNBA)
	}
	if hasAnyPhrase(norm, "stanley cup", "nhl champion", "hockey champion", "nhl championship") {
		return leagueConfigByLeague(espn.LeagueNHL)
	}
	if hasAnyPhrase(norm, "world series", "mlb champion", "mlb championship", "baseball champion") {
		return leagueConfigByLeague(espn.LeagueMLB)
	}
	if hasAnyPhrase(norm, "ncaa champion", "march madness winner", "college basketball champion") {
		return leagueConfigByLeague(espn.LeagueMensCollegeBball)
	}
	if hasAnyPhrase(norm, "college football champion", "cfp champion", "cfb champion") {
		return leagueConfigByLeague(espn.LeagueCollegeFootball)
	}
	return LeagueConfig{}, false
}
