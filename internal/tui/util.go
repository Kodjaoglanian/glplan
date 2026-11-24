package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return ansi.Truncate(s, n, "…")
}

func padRight(s string, n int) string {
	width := ansi.StringWidth(s)
	if width >= n {
		return truncate(s, n)
	}
	return s + strings.Repeat(" ", n-width)
}

func fit(s string, n int) string { return padRight(truncate(s, n), n) }

func wrap(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	words := strings.Fields(s)
	lines := []string{}
	cur := ""
	for _, w := range words {
		if utf8.RuneCountInString(w) > width {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			for len([]rune(w)) > width {
				r := []rune(w)
				lines = append(lines, string(r[:width]))
				w = string(r[width:])
			}
		}
		if cur == "" {
			cur = w
		} else if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= width {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func clipLines(lines []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	if len(lines) <= limit {
		return lines
	}
	out := append([]string{}, lines[:limit-1]...)
	out = append(out, faintStyle.Render("…"))
	return out
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "??"
	}
	if len(parts) == 1 {
		r := []rune(parts[0])
		return strings.ToUpper(string(r[:min(2, len(r))]))
	}
	first := []rune(parts[0])
	last := []rune(parts[len(parts)-1])
	if len(first) == 0 || len(last) == 0 {
		return "??"
	}
	return strings.ToUpper(string(first[0]) + string(last[0]))
}

func progressBar(done, total, width int) string {
	if width <= 0 || total <= 0 {
		return ""
	}
	filled := done * width / total
	if done > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + faintStyle.Render(strings.Repeat("░", width-filled))
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func countLabel(n int, singular, pluralForm string) string {
	return fmt.Sprintf("%d %s", n, plural(n, singular, pluralForm))
}

func daysUntil(t time.Time) int {
	now := time.Now()
	local := t.In(now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, now.Location())
	return int(dueDay.Sub(today).Hours() / 24)
}

func dueLabel(t time.Time) string {
	local := t.In(time.Now().Location())
	switch days := daysUntil(t); {
	case days < 0:
		return local.Format("02/01") + " atrasada"
	case days == 0:
		return "hoje"
	case days == 1:
		return "amanhã"
	default:
		return local.Format("02/01")
	}
}

func projectKeyFromName(name string) string {
	words := strings.Fields(name)
	var letters []rune
	if len(words) > 1 {
		for _, word := range words {
			for _, r := range word {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					letters = append(letters, unicode.ToUpper(r))
					break
				}
			}
			if len(letters) == 4 {
				break
			}
		}
	}
	if len(letters) < 2 {
		letters = nil
		for _, r := range name {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters = append(letters, unicode.ToUpper(r))
			}
			if len(letters) == 4 {
				break
			}
		}
	}
	return string(letters)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
