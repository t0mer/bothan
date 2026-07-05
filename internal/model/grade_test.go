package model

import "testing"

func TestGradeRank_Ordering(t *testing.T) {
	order := []string{"A+", "A", "A-", "B", "C", "D", "E", "F"}
	for i := 0; i < len(order)-1; i++ {
		if GradeRank(order[i]) <= GradeRank(order[i+1]) {
			t.Errorf("%s should rank above %s", order[i], order[i+1])
		}
	}
	if GradeRank("T") != GradeRankFail || GradeRank("M") != GradeRankFail {
		t.Errorf("T/M should be failing rank")
	}
	if GradeRank("") != GradeRankUnknown || GradeRank("Z") != GradeRankUnknown {
		t.Errorf("unknown grades should be rank %d", GradeRankUnknown)
	}
	if GradeRank("a+") != GradeRank("A+") {
		t.Errorf("grade rank should be case-insensitive")
	}
}

func TestLowestGrade(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"A+", "A", "B"}, "B"},
		{[]string{"A+"}, "A+"},
		{[]string{"A", "T"}, "T"},       // T is failing, lowest
		{[]string{"A", "", "A-"}, "A-"}, // empties skipped
		{[]string{}, ""},
		{[]string{"F", "A+"}, "F"},
	}
	for _, c := range cases {
		if got := LowestGrade(c.in); got != c.want {
			t.Errorf("LowestGrade(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLowerGrade(t *testing.T) {
	if !LowerGrade("B", "A") {
		t.Error("B should be lower than A")
	}
	if LowerGrade("A+", "A") {
		t.Error("A+ should not be lower than A")
	}
}
