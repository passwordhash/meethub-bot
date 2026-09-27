package domain

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

var planHeadings = []string{"сегодня", "на сегодня", "сделаю сегодня", "задачи на сегодня"}
var inlineSectionHeadings = map[string]struct{}{
	"завтра": {}, "вчера": {}, "сделано": {}, "итоги": {}, "блокеры": {}, "проблемы": {},
}

var headingMarkup = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
var unorderedList = regexp.MustCompile(`^\s*(?:[-*+]\s|\d+[.)]\s)`)

func ExtractPlans(reports []models.Report) []models.Plan {
	plans := make([]models.Plan, 0, len(reports))
	for _, report := range reports {
		text, ok := extractPlan(report.Text)
		if !ok {
			text = strings.TrimSpace(report.Text)
		}
		if text != "" {
			plans = append(plans, models.Plan{AuthorID: report.AuthorID, Author: report.Author, Text: text})
		}
	}
	return plans
}

func EligibleLeadIDs(reports []models.Report, previousLeadID int64) []int64 {
	ids := make([]int64, 0, len(reports))
	seen := make(map[int64]struct{}, len(reports))
	for _, report := range reports {
		if _, exists := seen[report.AuthorID]; exists {
			continue
		}
		seen[report.AuthorID] = struct{}{}
		ids = append(ids, report.AuthorID)
	}
	if len(ids) <= 1 {
		return ids
	}
	eligible := ids[:0]
	for _, id := range ids {
		if id != previousLeadID {
			eligible = append(eligible, id)
		}
	}
	return eligible
}

func extractPlan(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	var plan []string
	found := false
	for _, line := range lines {
		content := strings.TrimSpace(line)
		if body, ok := planHeadingBody(content); ok {
			if found {
				break
			}
			found = true
			if body != "" {
				plan = append(plan, body)
			}
			continue
		}
		if found && isSectionHeading(content) {
			break
		}
		if found {
			plan = append(plan, line)
		}
	}
	result := strings.TrimSpace(strings.Join(plan, "\n"))
	return result, found && result != ""
}

func planHeadingBody(line string) (string, bool) {
	line = headingMarkup.ReplaceAllString(line, "")
	line = strings.TrimSpace(strings.Trim(line, "*_`~"))
	for _, heading := range planHeadings {
		if !strings.HasPrefix(strings.ToLower(line), heading) {
			continue
		}
		remainder := line[len(heading):]
		if remainder == "" {
			return "", true
		}
		first, size := firstNonSpace(remainder)
		if !strings.ContainsRune(":：!！.,，—–-\u2014", first) {
			continue
		}
		return strings.TrimSpace(strings.TrimLeftFunc(remainder[size:], func(character rune) bool {
			return unicode.IsSpace(character) || strings.ContainsRune(":：!！.,，—–-\u2014", character)
		})), true
	}
	return "", false
}

func isSectionHeading(line string) bool {
	if line == "" || unorderedList.MatchString(line) {
		return false
	}
	if headingMarkup.MatchString(line) {
		return true
	}
	plain := strings.TrimSpace(strings.TrimLeft(line, "#"))
	if strings.HasSuffix(plain, ":") || strings.HasSuffix(plain, "：") {
		return true
	}
	separator := strings.IndexAny(plain, ":：")
	if separator <= 0 {
		return false
	}
	_, known := inlineSectionHeadings[strings.ToLower(strings.TrimSpace(plain[:separator]))]
	return known
}

func firstNonSpace(value string) (rune, int) {
	for index, character := range value {
		if !unicode.IsSpace(character) {
			return character, len(value[:index]) + len(string(character))
		}
	}
	return 0, len(value)
}
