package notifier

// MSTeams Adaptive Card types.
// Reference: https://learn.microsoft.com/en-us/microsoftteams/platform/task-modules-and-cards/cards/cards-format?tabs=adaptive-md%2Cdesktop%2Cdesktop1%2Cdesktop2%2Cconnector-html#mention-support-within-adaptive-cards

type mSTeamsAdaptiveCardMessage struct {
	Type        string       `json:"type"`
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	ContentType string       `json:"contentType"`
	Content     adaptiveCard `json:"content"`
}

type adaptiveCard struct {
	Schema  string        `json:"$schema,omitempty"`
	Type    string        `json:"type"`
	Version string        `json:"version"`
	Body    []interface{} `json:"body"`
	Actions []interface{} `json:"actions,omitempty"`
	MSTeams *mSTeamsInfo  `json:"msteams,omitempty"`
}

type mSTeamsInfo struct {
	Entities []mentionEntity `json:"entities"`
}

type mentionEntity struct {
	Type      string        `json:"type"`
	Text      string        `json:"text"`
	Mentioned mentionedUser `json:"mentioned"`
}

type textBlock struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Wrap   bool   `json:"wrap,omitempty"`
	Weight string `json:"weight,omitempty"`
	Size   string `json:"size,omitempty"`
}

type factSet struct {
	Type  string `json:"type"`
	Facts []fact `json:"facts"`
}

type fact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

type mentionedUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type actionOpenURL struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}
