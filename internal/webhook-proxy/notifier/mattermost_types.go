package notifier

// MattermostPayload is the top-level JSON body posted to a Mattermost incoming webhook.
// Mattermost accepts Slack-compatible attachments when the top-level text is empty.
type MattermostPayload struct {
	Text        string                   `json:"text"`
	Attachments []MattermostAttachment   `json:"attachments"`
}

// MattermostAttachment is a Slack-compatible attachment used by Mattermost.
// See: https://docs.mattermost.com/developer/message-attachments.html
type MattermostAttachment struct {
	Fallback string                   `json:"fallback"`
	Color    string                   `json:"color"`
	Title    string                   `json:"title"`
	TitleLink string                  `json:"title_link,omitempty"`
	Fields   []MattermostField        `json:"fields"`
}

// MattermostField is a single key-value field displayed in an attachment.
type MattermostField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}
