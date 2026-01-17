package structs

type Bot struct {
	BotID   string `json:"bot_id"`
	GitRepo string `json:"git_repo"`
	Version string `json:"version"`
}
