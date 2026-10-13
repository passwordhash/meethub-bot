package domain

import (
	"fmt"
	"html"
	"strings"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

var moscow = time.FixedZone("UTC+3", 3*60*60)

func RenderDaily(startsAt time.Time, lead *models.Report, mentionLead bool, meetingURL string) string {
	var message strings.Builder
	local := startsAt.In(moscow)
	fmt.Fprintf(&message, "📅 Дейли команды — %s, %s\n📞 Созвон в %s (UTC+3): %s", channelmodels.ShortWeekday(local.Weekday()), local.Format("02.01.2006"), local.Format("15:04"), html.EscapeString(meetingURL))
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
