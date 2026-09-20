package jev

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestQuestionsRoundTrip(t *testing.T) {
	want := Questions{
		"urgent": NoulQuestion{Instructions: "Urgent?", Criteria: &NoulCriteria{True: "Time sensitive", False: "Routine"}},
		"team":   ChoiceQuestion{Instructions: map[string]any{"task": "Route"}, Criteria: Options{"billing": "Payments", "other": ""}},
		"rating": ScoreQuestion{Instructions: []any{"Rate"}, Criteria: []string{"Low", "High"}},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"other":null`) {
		t.Fatalf("missing null option: %s", data)
	}
	var got Questions
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for id, q := range got {
		if q.Type() != want[id].Type() {
			t.Errorf("wrong type for %s", id)
		}
	}
}

func TestAnswersDecode(t *testing.T) {
	data := []byte(`{"model":"jev-latest","answers":{
 "urgent":{"type":"noul","noul":0.92},
 "team":{"type":"choice","choice":"billing","probabilities":{"billing":0.8,"other":0.2},"confidence":0.7},
 "rating":{"type":"score","score":0.6,"legend":{"0":"Low","1":"High"},"probabilities":{"0":0.4,"1":0.6},"confidence":0.2}
 },"usage":{"input_tokens":312,"output_tokens":48,"cost":0.01},"id":"gen-dec-1","provider":"TypeSafe"}`)
	var got Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := Answers{
		"urgent": NoulAnswer{Noul: 0.92},
		"team":   ChoiceAnswer{Choice: "billing", Probabilities: map[string]float64{"billing": 0.8, "other": 0.2}, Confidence: 0.7},
		"rating": ScoreAnswer{Score: 0.6, Legend: map[string]string{"0": "Low", "1": "High"}, Probabilities: map[string]float64{"0": 0.4, "1": 0.6}, Confidence: 0.2},
	}
	if !reflect.DeepEqual(got.Answers, want) {
		t.Fatalf("got %#v, want %#v", got.Answers, want)
	}
	if got.Model != "jev-latest" || got.Usage.InputTokens != 312 || got.Usage.OutputTokens != 48 || got.Usage.Cost != 0.01 || got.ID != "gen-dec-1" || got.Provider != "TypeSafe" {
		t.Fatalf("lost metadata: %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var again Response
	if err := json.Unmarshal(encoded, &again); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, again) {
		t.Fatalf("round trip changed answers: %+v", again)
	}
	for id, a := range got.Answers {
		if a.Type() != want[id].Type() {
			t.Errorf("wrong type for %s", id)
		}
	}
}

func TestInvalidTypedMaps(t *testing.T) {
	for _, data := range []string{
		`{"bad":{}}`, `{"bad":null}`, `{"bad":{"type":"future"}}`,
		`{"bad":{"type":12}}`, `[]`, `{"bad":`,
	} {
		t.Run(data, func(t *testing.T) {
			var questions Questions
			if err := json.Unmarshal([]byte(data), &questions); err == nil {
				t.Fatal("accepted invalid question")
			}
			var answers Answers
			if err := json.Unmarshal([]byte(data), &answers); err == nil {
				t.Fatal("accepted invalid answer")
			}
		})
	}
	for _, kind := range []string{"noul", "choice", "score"} {
		t.Run(kind, func(t *testing.T) {
			var answers Answers
			err := json.Unmarshal([]byte(`{"bad":{"type":"`+kind+`","`+kind+`":[]}}`), &answers)
			if err == nil || !strings.Contains(err.Error(), `answer "bad"`) {
				t.Fatalf("missing contextual error: %v", err)
			}
			var questions Questions
			err = json.Unmarshal([]byte(`{"bad":{"type":"`+kind+`","criteria":42}}`), &questions)
			if err == nil || !strings.Contains(err.Error(), `question "bad"`) {
				t.Fatalf("missing contextual error: %v", err)
			}
		})
	}
}

func TestNullMaps(t *testing.T) {
	for _, target := range []any{&Questions{}, &Answers{}, &Options{}} {
		if err := json.Unmarshal([]byte(`null`), target); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(target)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "null" {
			t.Fatalf("null became %s", data)
		}
	}
}

func TestResponseAnswer(t *testing.T) {
	resp := &Response{Answers: Answers{
		"urgent": NoulAnswer{Noul: 0.92},
		"team":   ChoiceAnswer{Choice: "billing", Confidence: 0.8},
		"mood":   ScoreAnswer{Score: 1.4, Confidence: 0.6},
	}}

	noul, err := resp.Answer[NoulAnswer]("urgent")
	if err != nil || noul.Noul != 0.92 {
		t.Fatalf("Answer[NoulAnswer] = %#v, %v", noul, err)
	}
	choice, err := resp.Answer[ChoiceAnswer]("team")
	if err != nil || choice.Choice != "billing" {
		t.Fatalf("Answer[ChoiceAnswer] = %#v, %v", choice, err)
	}
	score, err := resp.Answer[ScoreAnswer]("mood")
	if err != nil || score.Score != 1.4 {
		t.Fatalf("Answer[ScoreAnswer] = %#v, %v", score, err)
	}

	// A key that was never asked about.
	if _, err := resp.Answer[NoulAnswer]("missing"); err == nil ||
		!strings.Contains(err.Error(), `no answer for question "missing"`) {
		t.Fatalf("unexpected error for missing key: %v", err)
	}

	// The right key, the wrong type: an error, not a panic.
	_, err = resp.Answer[NoulAnswer]("team")
	if err == nil || !strings.Contains(err.Error(), "is a choice answer") {
		t.Fatalf("unexpected error for type mismatch: %v", err)
	}
}

func TestAnswerString(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer Answer
		want   string
	}{
		{
			name:   "noul",
			answer: NoulAnswer{Noul: 0.923},
			want:   "noul 0.92",
		},
		{
			name: "choice ranks by probability",
			answer: ChoiceAnswer{
				Choice:        "billing",
				Confidence:    0.8,
				Probabilities: map[string]float64{"other": 0.05, "billing": 0.75, "technical": 0.2},
			},
			want: `choice "billing" (confidence 0.80): billing 0.75, technical 0.20, other 0.05`,
		},
		{
			name:   "choice without a distribution",
			answer: ChoiceAnswer{Choice: "billing", Confidence: 0.8},
			want:   `choice "billing" (confidence 0.80)`,
		},
		{
			name: "score keeps rubric order and uses the legend",
			answer: ScoreAnswer{
				Score:         1.4,
				Confidence:    0.6,
				Legend:        map[string]string{"0": "Calm", "1": "Frustrated", "2": "Very angry"},
				Probabilities: map[string]float64{"2": 0.5, "0": 0.1, "1": 0.4},
			},
			want: "score 1.40 of 2 (confidence 0.60): Calm 0.10, Frustrated 0.40, Very angry 0.50",
		},
		{
			name: "score falls back to level indexes",
			answer: ScoreAnswer{
				Score:         0.5,
				Confidence:    0.5,
				Probabilities: map[string]float64{"1": 0.5, "0": 0.5},
			},
			want: "score 0.50 of 1 (confidence 0.50): 0 0.50, 1 0.50",
		},
		{
			name:   "score without a distribution",
			answer: ScoreAnswer{Score: 0.5, Confidence: 0.5},
			want:   "score 0.50 (confidence 0.50)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.answer.String(); got != tc.want {
				t.Errorf("String()\n got: %s\nwant: %s", got, tc.want)
			}
			// fmt must reach String rather than dumping the struct. Calling
			// String directly here would delete what this line checks.
			//nolint:gocritic // redundantSprint: going through fmt is the point.
			if got := fmt.Sprintf("%v", tc.answer); got != tc.want {
				t.Errorf("Sprintf(%%v) = %s, want %s", got, tc.want)
			}
		})
	}
}

// A whole map of answers prints one per line, sorted and aligned.
func TestAnswersString(t *testing.T) {
	answers := Answers{
		"urgent":      NoulAnswer{Noul: 0.92},
		"team":        ChoiceAnswer{Choice: "billing", Confidence: 0.8},
		"frustration": ScoreAnswer{Score: 1.4, Confidence: 0.6},
	}
	want := strings.Join([]string{
		`frustration: score 1.40 (confidence 0.60)`,
		`team:        choice "billing" (confidence 0.80)`,
		`urgent:      noul 0.92`,
	}, "\n")
	//nolint:gocritic // redundantSprint: going through fmt is the point.
	if got := fmt.Sprint(answers); got != want {
		t.Errorf("Sprint(Answers)\n got:\n%s\nwant:\n%s", got, want)
	}
	if got := (Answers{}).String(); got != "" {
		t.Errorf("empty Answers printed %q", got)
	}
}
