package bot

// botToken and miniAppURL are set once at startup.
var (
	botToken   string
	miniAppURL string
)

// SetBotToken stores the Telegram bot token (kept for potential future use).
func SetBotToken(token string) { botToken = token }

// SetMiniAppURL stores the GitHub Pages URL used in the settings keyboard.
func SetMiniAppURL(u string) { miniAppURL = u }

// GetMiniAppURL returns the configured Mini App URL.
func GetMiniAppURL() string { return miniAppURL }
