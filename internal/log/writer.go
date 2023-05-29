package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/oxygenpay/oxygen/internal/slack"
	"github.com/rs/zerolog"
)

// StdoutWriter default writer.
func StdoutWriter(pretty bool) io.Writer {
	os.Stderr = os.Stdout
	var output io.Writer = os.Stdout

	if pretty {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.StampMilli,
		}
	}

	return output
}

type slackWriter struct {
	webhookURL string
	level      zerolog.Level
}

func SlackWriter(webhookURL string, level zerolog.Level) zerolog.LevelWriter {
	if webhookURL == "" {
		return zerolog.MultiLevelWriter(io.Discard)
	}

	return &slackWriter{webhookURL, level}
}

func (w *slackWriter) Write(p []byte) (int, error) {
	w.send(p)
	return len(p), nil
}

func (w *slackWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if level < w.level {
		return len(p), nil
	}

	return w.Write(p)
}

// sending is not guarantied. This is a very primitive logger.
func (w *slackWriter) send(raw []byte) {
	if len(raw) == 0 {
		return
	}

	go func() {
		props := make(map[string]json.RawMessage)
		_ = json.Unmarshal(raw, &props)

		if len(props) == 0 {
			return
		}

		if skipSlack(props) {
			return
		}

		title := "_no message provided_"
		if _, ok := props["message"]; ok {
			title = string(props["message"])
			delete(props, "message")
		}

		if _, ok := props["error"]; ok {
			title = fmt.Sprintf("%s\n 🚨Error occurred ```\n%s\n```", title, props["error"])
			delete(props, "error")
		}

		if err := slack.SendWebhook(w.webhookURL, title, prettyPrint(props)); err != nil {
			fmt.Printf("Unable to send slack webhook: %s\n", err.Error())
		}
	}()
}

var omitErrors = map[string]struct{}{
	`"code=404, message=Not Found"`:          {},
	`"code=403, message=invalid csrf token"`: {},
	`"invalid UUID 'paymentId'"`:             {},
}

func skipSlack(props map[string]json.RawMessage) bool {
	errMessage := string(props["error"])
	if errMessage == "" {
		return false
	}

	_, shouldSkip := omitErrors[errMessage]

	return shouldSkip
}

func prettyPrint(data map[string]json.RawMessage) string {
	result, _ := json.MarshalIndent(data, "", "  ")

	return fmt.Sprintf("```\n%s\n```", string(result))
}
