package polymarket

// CategoryMeta describes a market category for display and filtering.
type CategoryMeta struct {
	Label    string `json:"label"`
	Emoji    string `json:"emoji"`
	Category string `json:"category"` // internal key used in user_settings.categories
}

// AllCategories lists every category the bot knows about.
// The key is used in user_settings.categories for per-user filtering.
var AllCategories = map[string]CategoryMeta{
	// Non-sport
	"politics":     {Label: "Политика", Emoji: "⚖️", Category: "politics"},
	"elections":    {Label: "Выборы", Emoji: "🗳️", Category: "elections"},
	"crypto":       {Label: "Крипто", Emoji: "💎", Category: "crypto"},
	"finance":      {Label: "Финансы", Emoji: "💵", Category: "finance"},
	"economy":      {Label: "Экономика", Emoji: "📈", Category: "economy"},
	"geopolitics":  {Label: "Геополитика", Emoji: "🌍", Category: "geopolitics"},
	"tech":         {Label: "Технологии", Emoji: "💻", Category: "tech"},
	"culture":      {Label: "Культура", Emoji: "🎬", Category: "culture"},
	"world":        {Label: "Мир", Emoji: "🌐", Category: "world"},
	// Sports
	"esports":      {Label: "Киберспорт", Emoji: "🎮", Category: "esports"},
	"hockey":       {Label: "Хоккей", Emoji: "🏒", Category: "hockey"},
	"basketball":   {Label: "Баскетбол", Emoji: "🏀", Category: "basketball"},
	"football":     {Label: "Футбол (амер.)", Emoji: "🏈", Category: "football"},
	"baseball":     {Label: "Бейсбол", Emoji: "⚾", Category: "baseball"},
	"fighting":     {Label: "MMA / Бокс", Emoji: "🥊", Category: "fighting"},
	"tennis":       {Label: "Теннис", Emoji: "🎾", Category: "tennis"},
	"cricket":      {Label: "Крикет", Emoji: "🏏", Category: "cricket"},
	"rugby":        {Label: "Регби", Emoji: "🏉", Category: "rugby"},
	"table_tennis": {Label: "Настольный теннис", Emoji: "🏓", Category: "table_tennis"},
	// Fallback
	"other": {Label: "Другое", Emoji: "🔔", Category: "other"},
}

// CategoryOrder defines the display order for the categories keyboard.
var CategoryOrder = [][]string{
	{"politics", "elections", "geopolitics"},
	{"crypto", "finance", "economy"},
	{"tech", "culture", "world"},
	{"esports", "hockey", "basketball"},
	{"football", "baseball", "fighting"},
	{"tennis", "cricket", "rugby", "table_tennis"},
	{"other"},
}

// BlackTags are Polymarket tag IDs whose markets should be silently dropped.
var BlackTags = []string{"1", "84", "1013", "100350", "306", "74", "103037"}

// BanWords in market titles cause the alert to be skipped.
var BanWords = []string{"solana", "bitcoin", "btc", "ethereum", "eth", "xrp", "ripple"}

// nonSportEmojis maps Polymarket tag slugs to emojis (non-sport categories).
var nonSportEmojis = map[string]string{
	"politics": "⚖️", "crypto": "💎", "finance": "💵", "geopolitics": "🌍",
	"tech": "💻", "culture": "🎬", "world": "🌐", "economy": "📈", "elections": "🗳️",
}

// sportTagIDs maps Polymarket numeric tag IDs to sport category keys.
var sportTagIDs = map[string]string{
	// Esports
	"64": "esports", "100780": "esports", "102366": "esports",
	"65": "esports", "101672": "esports",
	// Hockey
	"100088": "hockey", "102907": "hockey", "102909": "hockey",
	"102908": "hockey", "899": "hockey", "103666": "hockey", "103667": "hockey",
	// Basketball
	"28": "basketball", "103099": "basketball", "103097": "basketball",
	"103093": "basketball", "103098": "basketball", "103100": "basketball",
	"103094": "basketball", "103096": "basketball", "103095": "basketball",
	"102114": "basketball", "745": "basketball",
	// American football
	"450": "football", "100351": "football", "929": "football",
	"103347": "football", "100396": "football",
	// Baseball
	"100381": "baseball", "100021": "baseball",
	// Fighting
	"279": "fighting", "683": "fighting", "103213": "fighting",
	// Tennis
	"864": "tennis",
	// Cricket
	"517": "cricket", "102803": "cricket",
	// Other
	"100639": "other",
}

// seriesToSport maps series slug fragments (from market slugs) to sport category keys.
var seriesToSport = map[string]string{
	"counter-strike": "esports", "cs2": "esports",
	"dota-2": "esports", "dota2": "esports",
	"league-of-legends": "esports", "lol": "esports",
	"valorant": "esports", "val": "esports",
	"call-of-duty": "esports", "honor-of-kings": "esports",
	"rainbow-six-siege": "esports", "starcraft-2": "esports",
	"rocket-league": "esports", "mlbb": "esports",
	"overwatch": "esports",

	"nhl": "hockey", "ahl": "hockey", "khl": "hockey", "shl": "hockey",
	"dehl": "hockey", "cehl": "hockey",
	"mens-winter-olympics-hockey": "hockey", "womens-winter-olympics-hockey": "hockey",

	"nba": "basketball", "cwbb": "basketball", "wnba": "basketball",
	"ncaa-cbb": "basketball", "ncaab": "basketball",
	"euroleague-basketball": "basketball", "liga-endesa": "basketball",
	"basketball-series-a": "basketball",
	"lnb": "basketball", "pro-a": "basketball", "nbl": "basketball", "kbl": "basketball",

	"nfl": "football", "cfb": "football",

	"mlb": "baseball", "kbo": "baseball",

	"ufc": "fighting", "zuffa": "fighting",

	"atp": "tennis", "wta": "tennis",

	"wtt":               "table_tennis",
	"wtt-mens-singles":  "table_tennis", "wtt-womens-singles": "table_tennis",
	"wtt-mens-doubles":  "table_tennis", "wtt-womens-doubles": "table_tennis",
	"ittf": "table_tennis",

	"ipl": "cricket", "international-cricket": "cricket", "cricket-australia": "cricket",

	"rugby-six-nations": "rugby", "rugby-top-14": "rugby",
	"super-rugby-pacific": "rugby", "united-rugby-championship": "rugby",
}

// ResolveSportByTagID returns the sport category key for a Polymarket tag ID, or "".
func ResolveSportByTagID(tagID string) string {
	return sportTagIDs[tagID]
}

// ResolveSportBySlug checks if a market slug fragment matches a known sport series.
func ResolveSportBySlug(slug string) string {
	return seriesToSport[slug]
}

// NonSportEmoji returns the emoji for a non-sport tag slug, or "".
func NonSportEmoji(tagSlug string) string {
	return nonSportEmojis[tagSlug]
}
