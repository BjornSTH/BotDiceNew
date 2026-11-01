// Package dice разбирает и бросает выражения вида "2к6+1к4+3".
package dice

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

const (
	MaxDice     = 100       // кубов в одной группе ("100к6")
	MaxSides    = 1_000_000 // граней у куба
	MaxConstant = 1_000_000 // абсолютное значение модификатора
	MaxTerms    = 10        // слагаемых в выражении
)

// ErrNotExpression означает, что строка вообще не похожа на бросок
// (на такое бот молчит). Остальные ошибки показываются игроку.
var ErrNotExpression = errors.New("не выражение броска")

var (
	exprRe = regexp.MustCompile(`^(?:\d*d\d+|\d+)(?:[+-](?:\d*d\d+|\d+))*$`)
	termRe = regexp.MustCompile(`([+-]?)(?:(\d*)d(\d+)|(\d+))`)
)

// Term — одно слагаемое: группа кубов (Sides > 0) или константа.
type Term struct {
	Neg   bool
	Count int
	Sides int
	Const int
}

func (t Term) IsDice() bool { return t.Sides > 0 }

type Expr struct {
	Terms []Term
}

// Parse принимает "d" и "к" в любом регистре. Должен быть хотя бы один куб.
func Parse(s string) (Expr, error) {
	s = normalize(s)
	if !exprRe.MatchString(s) || !strings.Contains(s, "d") {
		return Expr{}, ErrNotExpression
	}
	matches := termRe.FindAllStringSubmatch(s, -1)
	if len(matches) > MaxTerms {
		return Expr{}, fmt.Errorf("не больше %d слагаемых", MaxTerms)
	}

	var e Expr
	for _, m := range matches {
		t := Term{Neg: m[1] == "-"}
		if m[4] != "" {
			c, err := atoiMax(m[4], MaxConstant)
			if err != nil {
				return Expr{}, fmt.Errorf("модификатор не больше %d", MaxConstant)
			}
			t.Const = c
		} else {
			t.Count = 1
			if m[2] != "" {
				c, err := atoiMax(m[2], MaxDice)
				if err != nil || c == 0 {
					return Expr{}, fmt.Errorf("кубов должно быть от 1 до %d", MaxDice)
				}
				t.Count = c
			}
			sides, err := atoiMax(m[3], MaxSides)
			if err != nil || sides == 0 {
				return Expr{}, fmt.Errorf("граней должно быть от 1 до %d", MaxSides)
			}
			t.Sides = sides
		}
		e.Terms = append(e.Terms, t)
	}
	return e, nil
}

func normalize(s string) string {
	return strings.NewReplacer("к", "d", "К", "d", "D", "d").Replace(strings.TrimSpace(s))
}

func atoiMax(s string, max int) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n > max {
		return 0, errors.New("out of range")
	}
	return n, nil
}

// String возвращает каноничную запись с русской "к": "2к6+к4-1".
func (e Expr) String() string {
	var sb strings.Builder
	for i, t := range e.Terms {
		if t.Neg {
			sb.WriteByte('-')
		} else if i > 0 {
			sb.WriteByte('+')
		}
		if !t.IsDice() {
			sb.WriteString(strconv.Itoa(t.Const))
			continue
		}
		if t.Count > 1 {
			sb.WriteString(strconv.Itoa(t.Count))
		}
		sb.WriteString("к")
		sb.WriteString(strconv.Itoa(t.Sides))
	}
	return sb.String()
}

// Rand возвращает равномерное число из [0, n).
type Rand interface {
	IntN(n int) int
}

// Crypto берёт каждое значение напрямую из системного CSPRNG: у бросков нет
// общей предсказуемой последовательности, и каждый бросок независим.
var Crypto Rand = cryptoRand{}

type cryptoRand struct{}

func (cryptoRand) IntN(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err) // crypto/rand.Reader не возвращает ошибок с Go 1.24
	}
	return int(v.Int64())
}

type TermResult struct {
	Term
	Values []int
}

type Result struct {
	Expr  Expr
	Terms []TermResult
	Total int
}

func (e Expr) Roll(r Rand) Result {
	res := Result{Expr: e}
	for _, t := range e.Terms {
		tr := TermResult{Term: t}
		sum := t.Const
		if t.IsDice() {
			tr.Values = make([]int, t.Count)
			sum = 0
			for i := range tr.Values {
				tr.Values[i] = r.IntN(t.Sides) + 1
				sum += tr.Values[i]
			}
		}
		if t.Neg {
			sum = -sum
		}
		res.Total += sum
		res.Terms = append(res.Terms, tr)
	}
	return res
}

// Details расписывает бросок: "9 +16", "4, 5, 3", "[4, 3] +[2] -1".
// Если групп кубов несколько, каждая берётся в скобки.
func (r Result) Details() string {
	diceGroups := 0
	for _, t := range r.Terms {
		if t.IsDice() {
			diceGroups++
		}
	}

	var sb strings.Builder
	for i, t := range r.Terms {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if t.Neg {
			sb.WriteByte('-')
		} else if i > 0 {
			sb.WriteByte('+')
		}
		if !t.IsDice() {
			sb.WriteString(strconv.Itoa(t.Const))
			continue
		}
		vals := make([]string, len(t.Values))
		for j, v := range t.Values {
			vals[j] = formatValue(v, t.Sides)
		}
		joined := strings.Join(vals, ", ")
		if diceGroups > 1 || i > 0 || (t.Neg && len(vals) > 1) {
			joined = "[" + joined + "]"
		}
		sb.WriteString(joined)
	}
	return sb.String()
}

func formatValue(v, sides int) string {
	s := strconv.Itoa(v)
	if sides == 20 {
		switch v {
		case 20:
			s += "🔥"
		case 1:
			s += "💀"
		}
	}
	return s
}

// String — итог с расшифровкой: "25 (9 +16)".
func (r Result) String() string {
	return fmt.Sprintf("%d (%s)", r.Total, r.Details())
}
