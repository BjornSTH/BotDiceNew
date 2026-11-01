package main

import (
	"strings"
	"testing"
)

// fixed всегда выдаёт одно значение куба (если оно не больше числа граней).
type fixed int

func (f fixed) IntN(n int) int { return min(int(f), n) - 1 }

// cycle выдаёт значения по кругу.
type cycle struct {
	vals []int
	i    int
}

func (c *cycle) IntN(n int) int {
	v := c.vals[c.i%len(c.vals)]
	c.i++
	return v - 1
}

func TestBuildReply(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"/к20", "Sx:\n>> бросок к20: 10 (10)"},
		{"/d20@RoflsCube20_bot", "Sx:\n>> бросок к20: 10 (10)"},
		{"/roll 3d7", "Sx:\n>> бросок 3к7: 21 (7, 7, 7)"},
		{"/roll", "Sx:\n>> бросок к20: 10 (10)"},
		{"к20\nd12-4", "Sx:\n>> бросок к20: 10 (10)\n>> бросок к12-4: 6 (10 -4)"},
		{"к20-15", "Sx:\n>> бросок к20-15: -5 (10 -15)"},
		{"1000d6", "Sx:\n>> 1000d6: ошибка — кубов должно быть от 1 до 100"},
		{"привет всем", ""},
		{"пр всем", ""},
		{"// к20", ""},
		{`\\ к20`, ""},
		{"20 гоблинов", ""},
	}
	for _, c := range cases {
		if got := buildReply(c.text, "Sx", fixed(10)); got != c.want {
			t.Errorf("buildReply(%q):\n got %q\nwant %q", c.text, got, c.want)
		}
	}
}

func TestAdvantage(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"пр к20+5", ">> преимущество к20+5: 22 — из 22 (17 +5) и 12 (7 +5)"},
		{"пм к20+5", ">> помеха к20+5: 12 — из 12 (7 +5) и 22 (17 +5)"},
		{"пр к20 к20", ">> преимущество к20: 17 — из 17 (17) и 7 (7)"},
		{"пм 1к4-10 1к4-10", ">> помеха к4-10: -3 — из -3 (7 -10) и 7 (17 -10)"},
	}
	for _, c := range cases {
		got := buildReply(c.text, "Sx", &cycle{vals: []int{17, 7}})
		if got != "Sx:\n"+c.want {
			t.Errorf("buildReply(%q):\n got %q\nwant %q", c.text, got, "Sx:\n"+c.want)
		}
	}

	// "пр к20 к6" — повтор не того же выражения, к6 бросается отдельно.
	got := buildReply("пр к20 к6", "Sx", fixed(3))
	if strings.Count(got, ">>") != 2 {
		t.Errorf("пр к20 к6: %q", got)
	}
}

func TestReplyLimits(t *testing.T) {
	got := buildReply(strings.Repeat("100к1000000 ", 30), "Sx", fixed(999999))
	if len(got) > maxReplyBytes+100 {
		t.Errorf("ответ слишком длинный: %d байт", len(got))
	}
	if !strings.Contains(got, "не поместились") {
		t.Errorf("нет пометки об обрезке: %q", got[len(got)-80:])
	}
}
