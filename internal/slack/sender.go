package slack

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/oxygenpay/oxygen/internal/util"
)

// SendWebhook sends message to Slack channel based.
// See https://api.slack.com/messaging/webhooks. Each string is markdown-compatible.
func SendWebhook(webhookURL string, markdownSections ...string) error {
	markdownSections = util.FilterSlice(markdownSections, func(s string) bool { return s != "" })

	sections := util.MapSlice(markdownSections, slackSection)

	body := `{ "blocks": [` + strings.Join(sections, ", ") + `] }`

	//nolint:gosec
	res, err := http.Post(webhookURL, "application/json", strings.NewReader(body))
	if err != nil {
		return err
	}

	_ = res.Body.Close()

	return nil
}

func slackSection(content string) string {
	return fmt.Sprintf(`{
		"type": "section",
		"text": { "type": "mrkdwn", "text": %q }
	}`, content)
}
