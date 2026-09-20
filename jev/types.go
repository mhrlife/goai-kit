// Package jev holds the wire types of TypeSafe's Jev decision model, as served by
// OpenRouter at POST https://openrouter.ai/api/alpha/decisions.
//
// Jev cannot generate text. A request carries a state and a map of typed questions;
// each question comes back answered with one of three primitives: noul (a yes/no
// probability), choice (one option out of the set you enumerate, with the whole
// distribution and a confidence) or score (a probability-weighted position on a
// rubric). Reference: https://docs.typesafe.ai/api.md
package jev

import (
	"encoding/json"
	"fmt"
	"time"
)

// Type is the primitive a question is asked in, and the one its answer comes back in.
type Type string

const (
	Noul   Type = "noul"
	Choice Type = "choice"
	Score  Type = "score"
)

// Request is the body of an evaluation call.
type Request struct {
	Model     string    `json:"model"`
	State     any       `json:"state"`
	Questions Questions `json:"questions"`
}

// Response is the body that comes back. One answer per question, under the same key.
type Response struct {
	Model   string  `json:"model"`
	Answers Answers `json:"answers"`
	Usage   Usage   `json:"usage"`
	// ID and Provider are sent by OpenRouter only; TypeSafe leaves them empty.
	// ID identifies the generation in OpenRouter's logs.
	ID       string `json:"id,omitempty"`
	Provider string `json:"provider,omitempty"`
	// Latency is measured by this package around the HTTP round trip, because
	// neither provider reports one. It therefore includes network time, and is
	// zero on a Response that was unmarshaled rather than fetched.
	Latency time.Duration `json:"-"`
}

// Usage reports the tokens the request cost. Cost is OpenRouter's, in dollars;
// TypeSafe does not price the call in its response, so it stays zero there.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Cost         float64 `json:"cost,omitempty"`
}

// Question is one of NoulQuestion, ChoiceQuestion or ScoreQuestion. Implementations
// marshal their own "type" field, so a question is always written in full.
type Question interface {
	json.Marshaler
	Type() Type
}

// Instructions is what the model should decide: a string, or structured data
// (object or array) when plain text is not enough.
type Instructions = any

// NoulQuestion asks a yes/no question. The answer is the probability of yes.
type NoulQuestion struct {
	Instructions Instructions  `json:"instructions"`
	Criteria     *NoulCriteria `json:"criteria,omitempty"`
}

// NoulCriteria optionally spells out what a yes and a no mean.
type NoulCriteria struct {
	True  string `json:"true,omitempty"`
	False string `json:"false,omitempty"`
}

func (NoulQuestion) Type() Type { return Noul }

func (q NoulQuestion) MarshalJSON() ([]byte, error) {
	type fields NoulQuestion // sheds the method set, so this does not recurse
	return json.Marshal(struct {
		Type Type `json:"type"`
		fields
	}{Type: Noul, fields: fields(q)})
}

// ChoiceQuestion picks one of Criteria's options. The choice is relative: the model
// picks something even when every option is bad, so offer a "none" option wherever
// the right answer may be missing.
type ChoiceQuestion struct {
	Instructions Instructions `json:"instructions"`
	Criteria     Options      `json:"criteria"`
}

func (ChoiceQuestion) Type() Type { return Choice }

func (q ChoiceQuestion) MarshalJSON() ([]byte, error) {
	type fields ChoiceQuestion
	return json.Marshal(struct {
		Type Type `json:"type"`
		fields
	}{Type: Choice, fields: fields(q)})
}

// Options maps each option to the rubric text describing it. The wire format allows
// null for an option that needs no description; an empty string is written as null
// and null reads back as an empty string.
type Options map[string]string

func (o Options) MarshalJSON() ([]byte, error) {
	if o == nil {
		return []byte("null"), nil
	}
	m := make(map[string]*string, len(o))
	for option, description := range o {
		if description == "" {
			m[option] = nil
			continue
		}
		m[option] = &description
	}
	return json.Marshal(m)
}

func (o *Options) UnmarshalJSON(data []byte) error {
	var m map[string]*string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m == nil {
		*o = nil
		return nil
	}
	out := make(Options, len(m))
	for option, description := range m {
		if description != nil {
			out[option] = *description
		} else {
			out[option] = ""
		}
	}
	*o = out
	return nil
}

// ScoreQuestion rates the state along an ordered rubric of at least two levels.
type ScoreQuestion struct {
	Instructions Instructions `json:"instructions"`
	Criteria     []string     `json:"criteria"`
}

func (ScoreQuestion) Type() Type { return Score }

func (q ScoreQuestion) MarshalJSON() ([]byte, error) {
	type fields ScoreQuestion
	return json.Marshal(struct {
		Type Type `json:"type"`
		fields
	}{Type: Score, fields: fields(q)})
}

// Questions is the map sent in a request. The keys are yours; the answers come back
// under the same ones. The keys are not shown to the model.
type Questions map[string]Question

func (qs *Questions) UnmarshalJSON(data []byte) error {
	raw, err := rawMap(data)
	if err != nil {
		return err
	}
	if raw == nil {
		*qs = nil
		return nil
	}
	out := make(Questions, len(raw))
	for id, msg := range raw {
		q, err := decodeQuestion(msg)
		if err != nil {
			return fmt.Errorf("question %q: %w", id, err)
		}
		out[id] = q
	}
	*qs = out
	return nil
}

func decodeQuestion(data []byte) (Question, error) {
	t, err := peekType(data)
	if err != nil {
		return nil, err
	}
	switch t {
	case Noul:
		var q NoulQuestion
		if err := json.Unmarshal(data, &q); err != nil {
			return nil, err
		}
		return q, nil
	case Choice:
		var q ChoiceQuestion
		if err := json.Unmarshal(data, &q); err != nil {
			return nil, err
		}
		return q, nil
	case Score:
		var q ScoreQuestion
		if err := json.Unmarshal(data, &q); err != nil {
			return nil, err
		}
		return q, nil
	default:
		return nil, fmt.Errorf("unknown question type %q", t)
	}
}

// Answer is one of NoulAnswer, ChoiceAnswer or ScoreAnswer.
type Answer interface {
	json.Marshaler
	Type() Type
}

// NoulAnswer is the yes/no answer, from 0 (no) to 1 (yes).
type NoulAnswer struct {
	Noul float64 `json:"noul"`
}

func (NoulAnswer) Type() Type { return Noul }

func (a NoulAnswer) MarshalJSON() ([]byte, error) {
	type fields NoulAnswer
	return json.Marshal(struct {
		Type Type `json:"type"`
		fields
	}{Type: Noul, fields: fields(a)})
}

// ChoiceAnswer carries the picked option, the probability of every option (they sum
// to 1) and a confidence derived from that distribution.
type ChoiceAnswer struct {
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

func (ChoiceAnswer) Type() Type { return Choice }

func (a ChoiceAnswer) MarshalJSON() ([]byte, error) {
	type fields ChoiceAnswer
	return json.Marshal(struct {
		Type Type `json:"type"`
		fields
	}{Type: Choice, fields: fields(a)})
}

// ScoreAnswer carries the probability-weighted value across the levels, which can
// land between them. Legend and Probabilities are keyed by level index as a string.
type ScoreAnswer struct {
	Score         float64            `json:"score"`
	Legend        map[string]string  `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

func (ScoreAnswer) Type() Type { return Score }

func (a ScoreAnswer) MarshalJSON() ([]byte, error) {
	type fields ScoreAnswer
	return json.Marshal(struct {
		Type Type `json:"type"`
		fields
	}{Type: Score, fields: fields(a)})
}

// Answers is the map returned in a response, keyed by the question ids of the request.
type Answers map[string]Answer

func (as *Answers) UnmarshalJSON(data []byte) error {
	raw, err := rawMap(data)
	if err != nil {
		return err
	}
	if raw == nil {
		*as = nil
		return nil
	}
	out := make(Answers, len(raw))
	for id, msg := range raw {
		a, err := decodeAnswer(msg)
		if err != nil {
			return fmt.Errorf("answer %q: %w", id, err)
		}
		out[id] = a
	}
	*as = out
	return nil
}

func decodeAnswer(data []byte) (Answer, error) {
	t, err := peekType(data)
	if err != nil {
		return nil, err
	}
	switch t {
	case Noul:
		var a NoulAnswer
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		return a, nil
	case Choice:
		var a ChoiceAnswer
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		return a, nil
	case Score:
		var a ScoreAnswer
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		return a, nil
	default:
		return nil, fmt.Errorf("unknown answer type %q", t)
	}
}

func rawMap(data []byte) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func peekType(data []byte) (Type, error) {
	var head struct {
		Type Type `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return "", err
	}
	if head.Type == "" {
		return "", fmt.Errorf("missing type field")
	}
	return head.Type, nil
}

var (
	_ Question = NoulQuestion{}
	_ Question = ChoiceQuestion{}
	_ Question = ScoreQuestion{}
	_ Answer   = NoulAnswer{}
	_ Answer   = ChoiceAnswer{}
	_ Answer   = ScoreAnswer{}
)
