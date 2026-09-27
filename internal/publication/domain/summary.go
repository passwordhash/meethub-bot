package domain

import (
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

const telegramMessageLimit = 4096

func RenderSummary(plans []models.Plan) []string {
	var messages []string
	for _, plan := range plans {
		label := strings.TrimSpace(plan.Author)
		if label == "" || utf16Length(label+":\n") >= telegramMessageLimit {
			label = fmt.Sprintf("Участник %d", plan.AuthorID)
		}
		continuation := false
		text := plan.Text
		for {
			prefix := label + ":\n"
			if continuation {
				prefix = label + " (продолжение):\n"
			}
			capacity := telegramMessageLimit - utf16Length(prefix)
			part, rest := takeUTF16(text, capacity)
			messages = append(messages, prefix+part)
			if rest == "" {
				break
			}
			text = rest
			continuation = true
		}
	}
	return messages
}

func takeUTF16(text string, capacity int) (string, string) {
	units := 0
	for index, character := range text {
		characterUnits := len(utf16.Encode([]rune{character}))
		if units+characterUnits > capacity {
			return text[:index], text[index:]
		}
		units += characterUnits
	}
	return text, ""
}

func utf16Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}
