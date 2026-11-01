package dice

import (
	"errors"
	"testing"
)

// seq выдаёт заранее заданные значения кубов (1..sides).
type seq []int

func (s *seq) IntN(n int) int {
	v := (*s)[0]
	*s = (*s)[1:]
	return v - 1
}

func TestParse(t *testing.T) {
	ok := map[string]string{
		"d20":      "к20",
		"К20":      "к20",
		"1d20":     "к20",
		"3к7":      "3к7",
		"к20+16":   "к20+16",
		"d12-4":    "к12-4",
		"2d6+1d4":  "2к6+к4",
		"к20-15+2": "к20-15+2",
		"5+к6":     "5+к6",
		"к1":       "к1",
	}
	for in, want := range ok {
		e, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if e.String() != want {
			t.Errorf("Parse(%q) = %q, want %q", in, e, want)
		}
	}

	for _, in := range []string{"", "roll", "20", "5+3", "-2к6", "d", "к20+", "2d6++1", "привет", "d20x"} {
		if _, err := Parse(in); !errors.Is(err, ErrNotExpression) {
			t.Errorf("Parse(%q) err = %v, want ErrNotExpression", in, err)
		}
	}

	for _, in := range []string{"d0", "0d6", "101d6", "d1000001", "99999999999999999999d6", "d20+1000001",
		"d1+d1+d1+d1+d1+d1+d1+d1+d1+d1+d1"} {
		if _, err := Parse(in); err == nil || errors.Is(err, ErrNotExpression) {
			t.Errorf("Parse(%q) err = %v, want limit error", in, err)
		}
	}
}

func TestRoll(t *testing.T) {
	cases := []struct {
		expr string
		dice seq
		want string
	}{
		{"к20+16", seq{9}, "25 (9 +16)"},
		{"к20-15", seq{5}, "-10 (5 -15)"},
		{"3к7", seq{4, 5, 3}, "12 (4, 5, 3)"},
		{"2к20", seq{20, 1}, "21 (20🔥, 1💀)"},
		{"2к6+к4-1", seq{4, 3, 2}, "8 ([4, 3] +[2] -1)"},
		{"5-2к6", seq{1, 2}, "2 (5 -[1, 2])"},
	}
	for _, c := range cases {
		e, err := Parse(c.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.expr, err)
		}
		if got := e.Roll(&c.dice).String(); got != c.want {
			t.Errorf("%s: got %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestCryptoRange(t *testing.T) {
	seen := map[int]bool{}
	for range 2000 {
		v := Crypto.IntN(6)
		if v < 0 || v >= 6 {
			t.Fatalf("IntN(6) = %d", v)
		}
		seen[v] = true
	}
	if len(seen) != 6 {
		t.Errorf("за 2000 бросков к6 выпали не все грани: %v", seen)
	}
	if Crypto.IntN(1) != 0 {
		t.Error("IntN(1) != 0")
	}
}
