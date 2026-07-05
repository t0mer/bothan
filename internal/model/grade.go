package model

import "strings"

// Canonical SSL Labs grade ordering used everywhere (rules, comparison,
// metrics). Higher rank is a better grade.
//
//	A+ > A > A- > B > C > D > E > F   (8..1)
//	T (trust/chain issue) and M (hostname mismatch) are failing/alert (0)
//	unknown / none is -1
const (
	GradeRankFail    = 0  // T, M
	GradeRankUnknown = -1 // none / empty / unrecognised
)

var gradeRanks = map[string]int{
	"A+": 8,
	"A":  7,
	"A-": 6,
	"B":  5,
	"C":  4,
	"D":  3,
	"E":  2,
	"F":  1,
	"T":  GradeRankFail,
	"M":  GradeRankFail,
}

// GradeRank returns the numeric rank for a grade per the canonical ordering.
func GradeRank(grade string) int {
	if r, ok := gradeRanks[normalizeGrade(grade)]; ok {
		return r
	}
	return GradeRankUnknown
}

// LowerGrade reports whether grade a ranks strictly below grade b.
func LowerGrade(a, b string) bool { return GradeRank(a) < GradeRank(b) }

// LowestGrade returns the worst (lowest-ranked) grade among the given grades,
// which is Bothan's overall host grade for a scan. Empty input yields "".
func LowestGrade(grades []string) string {
	worst := ""
	worstRank := GradeRankUnknown + 1 // above unknown so the first grade wins
	first := true
	for _, g := range grades {
		g = normalizeGrade(g)
		if g == "" {
			continue
		}
		r := GradeRank(g)
		if first || r < worstRank {
			worst, worstRank, first = g, r, false
		}
	}
	return worst
}

func normalizeGrade(g string) string {
	return strings.ToUpper(strings.TrimSpace(g))
}
