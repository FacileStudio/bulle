// Package jev is the decision client for two surfaces that share one contract:
// TypeSafe System One and the OpenRouter Decisions API. Typed questions in,
// calibrated decisions out, and no generated text at all. It carries no SDK
// dependency — the wire format is three fields and a JSON body — and the same
// Question, Answer and Usage types decode both surfaces, so a caller can swap
// the endpoint without touching its parsing.
//
// System One lives at POST {base}/v1/systemone on https://api.typesafe.ai with
// a TypeSafe key. The Decisions API lives at POST {base}/api/alpha/decisions on
// https://openrouter.ai with an OpenRouter key, and it is the surface for
// Cloudflare's clef as well as for JEV through OpenRouter. Both take {model,
// state, questions}; both answer with {answers, usage} whose answers carry the
// same choice/noul/score shapes. Decisions additionally reports usage.cost.
//
// JEV is early access. Model names, availability and rate limits can move, so
// the caller treats any failure as a reason to fall back, never as an error to
// surface as a broken conversation. See internal/compaction's judge.
package jev

// Question is one typed ask. Instructions and Criteria are `any` on purpose: the
// API takes a string or a list of strings for instructions, and a criteria's
// shape follows the question type — a choice takes an object keyed by option and
// holding what belongs to it, where a score takes a list of the level
// descriptions. Narrowing either here would mean this client owning a shape the
// vendor defines, and a criteria that encodes as the wrong shape is a 422 the
// endpoint reports and a stub server does not.
type Question struct {
	Type         string   `json:"type"`
	Instructions any      `json:"instructions"`
	Criteria     any      `json:"criteria,omitempty"`
	Options      []string `json:"options,omitempty"`
}

// Answer is one question's result. Answer is a "choice": the picked option, the
// calibrated confidence and the option probabilities. Noul carries the other
// primitive's single number and stays zero for a choice.
type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	Noul          float64            `json:"noul"`
}

// Usage is what one call billed. Output is free on JEV, but the field is read
// anyway so the client never has to guess which side it was on. Cost is filled
// in by the OpenRouter Decisions surface, which reports it; TypeSafe direct
// never does, and the field stays zero.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Cost         float64 `json:"cost,omitempty"`
}

// Response is one Evaluate call's answer: the model that served it, every
// question's result keyed by the name it was asked under, and the usage. ID and
// Provider are named by the Decisions surface and are empty on System One.
type Response struct {
	Model    string            `json:"model"`
	Answers  map[string]Answer `json:"answers"`
	Usage    Usage             `json:"usage"`
	ID       string            `json:"id,omitempty"`
	Provider string            `json:"provider,omitempty"`
}

// request is the wire body. Every question is evaluated in parallel against the
// same state in one call, which is the fact the whole judge design leans on.
//
// State is `any` on purpose. Both surfaces accept a plain string and the
// Decisions API also accepts a JSON object or array, which matters because a
// decision model reads a structured state and a text blob differently. Typing
// it here would mean this client deciding what the vendors allow.
type request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}
