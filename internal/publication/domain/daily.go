package domain

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

var moscow = time.FixedZone("UTC+3", 3*60*60)

func RenderDaily(date time.Time, lead *models.Report, mentionLead bool, meetingURL string) string {
	var message strings.Builder
	fmt.Fprintf(&message, "📅 Дейли команды — %s\n📞 Созвон в 10:00 (UTC+3): %s", date.In(moscow).Format("02.01.2006"), html.EscapeString(meetingURL))
	if lead != nil {
		message.WriteString("\n👤 Ведущий: ")
		name := html.EscapeString(lead.Author)
		if mentionLead && lead.AuthorID != 0 {
			fmt.Fprintf(&message, "<a href=\"tg://user?id=%d\">%s</a>", lead.AuthorID, name)
		} else {
			message.WriteString(name)
		}
	}
	message.WriteString("\n\nПишите отчёты в тред 👇")
	return message.String()
}
