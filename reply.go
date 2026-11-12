package main

import (
	"errors"
	"fmt"
	"html"
	"strings"

	"botdice/internal/dice"
)

const (
	maxRollsPerMessage = 20
	// Лимит Telegram — 4096 символов без учёта HTML-тегов; считаем в байтах
	// вместе с тегами, так что запас с лихвой.
	maxReplyBytes = 4000
)

const helpText = `Бросок кубиков: напишите выражение, можно несколько через пробел или с новой строки.

к20, d20, 3к6, к20+5, 2к6+1к4+3, к12-4
пр к20+5 или pr d20+5 — с преимуществом (лучший из двух)
пм к20+5 или pm d20+5 — с помехой (худший из двух)

Сообщения, начинающиеся с // или \\, бот игнорирует.`

// buildReply превращает текст сообщения в ответ бота в HTML-разметке Telegram.
// Пустая строка — в сообщении нечего бросать, отвечать не нужно.
func buildReply(text, player string, r dice.Rand) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "//") || strings.HasPrefix(text, `\\`) {
		return ""
	}

	tokens := strings.Fields(text)
	for i, tok := range tokens {
		tokens[i] = cleanToken(tok)
	}
	if len(tokens) > 0 {
		switch tokens[0] {
		case "start", "help":
			return helpText
		case "roll":
			if len(tokens) == 1 {
				tokens = append(tokens, "к20")
			}
		}
	}

	var lines []string
	for i := 0; i < len(tokens) && len(lines) < maxRollsPerMessage; i++ {
		tok := tokens[i]
		mode := ""
		switch tok {
		case "пр", "pr", "adv":
			mode = "преимущество"
		case "пм", "pm", "dis":
			mode = "помеха"
		}

		if mode == "" {
			expr, err := dice.Parse(tok)
			if errors.Is(err, dice.ErrNotExpression) {
				continue
			}
			if err != nil {
				lines = append(lines, fmt.Sprintf(">> %s: ошибка — %v", html.EscapeString(tok), err))
				continue
			}
			lines = append(lines, fmt.Sprintf(">> бросок %s: %s", expr, formatResult(expr.Roll(r))))
			continue
		}

		if i+1 >= len(tokens) {
			continue
		}
		expr, err := dice.Parse(tokens[i+1])
		if errors.Is(err, dice.ErrNotExpression) {
			continue // "пр" как обычное слово, а не команда
		}
		i++
		if err != nil {
			lines = append(lines, fmt.Sprintf(">> %s %s: ошибка — %v", mode, html.EscapeString(tokens[i]), err))
			continue
		}
		// Старый синтаксис "пр к20 к20": повтор того же выражения поглощаем.
		if i+1 < len(tokens) {
			if next, err := dice.Parse(tokens[i+1]); err == nil && next.String() == expr.String() {
				i++
			}
		}
		a, b := expr.Roll(r), expr.Roll(r)
		best, other := a, b
		if (mode == "преимущество") == (b.Total > a.Total) {
			best, other = b, a
		}
		lines = append(lines, fmt.Sprintf(">> %s %s: <b>%d</b> — из %s и %s", mode, expr, best.Total, best, other))
	}

	if len(lines) == 0 {
		return ""
	}

	reply := html.EscapeString(player) + ":"
	for i, line := range lines {
		if len(reply)+1+len(line) > maxReplyBytes {
			reply += fmt.Sprintf("\n…ещё %d бросков не поместились в сообщение", len(lines)-i)
			break
		}
		reply += "\n" + line
	}
	return reply
}

// formatResult выделяет итог жирным: "<b>25</b> (9 +16)".
func formatResult(r dice.Result) string {
	return fmt.Sprintf("<b>%d</b> (%s)", r.Total, r.Details())
}

// cleanToken убирает "/" команды, суффикс "@имябота" и знаки препинания по краям.
func cleanToken(tok string) string {
	tok = strings.TrimPrefix(tok, "/")
	if at := strings.IndexByte(tok, '@'); at >= 0 {
		tok = tok[:at]
	}
	return strings.ToLower(strings.Trim(tok, ".,;:!?()"))
}
