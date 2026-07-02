//go:build e2e

package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Status values used throughout the report.
const (
	StatusPass = "pass"
	StatusFail = "fail"
	StatusSkip = "skip"
)

// StepTiming records how long a named step took.
type StepTiming struct {
	Step       string `json:"step"`
	DurationMS int64  `json:"duration_ms"`
}

// NamedVerdict pairs an LLM comparison label with its verdict.
type NamedVerdict struct {
	Comparison string   `json:"comparison"` // e.g. "ddn-vs-es"
	Verdict    *Verdict `json:"verdict,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// QueryReport is the L4 outcome for a single query.
type QueryReport struct {
	Name     string         `json:"name"`
	Layer    string         `json:"layer"` // "L4"
	Target   string         `json:"target_index"`
	Status   string         `json:"status"`
	Message  string         `json:"message,omitempty"`
	Verdicts []NamedVerdict `json:"verdicts,omitempty"`

	// Payloads are attached on failure (and when regenerating goldens) for debugging.
	DDNPayload string `json:"ddn_payload,omitempty"`
	ESPayload  string `json:"es_payload,omitempty"`
	GoldenDDN  string `json:"golden_ddn,omitempty"`
	GoldenES   string `json:"golden_es,omitempty"`

	Timings []StepTiming `json:"timings,omitempty"`
}

// CaseReport is the outcome for a whole case (L3 + all its L4 queries).
type CaseReport struct {
	Name           string        `json:"name"`
	Status         string        `json:"status"`
	Message        string        `json:"message,omitempty"`
	SchemaLayer    string        `json:"schema_layer"` // "L3"
	SchemaStatus   string        `json:"schema_status"`
	SchemaProblems []string      `json:"schema_problems,omitempty"`
	Queries        []QueryReport `json:"queries"`
	Timings        []StepTiming  `json:"timings,omitempty"`
	DurationMS     int64         `json:"duration_ms"`
}

// Report is the top-level e2e report.
type Report struct {
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	Env        map[string]string `json:"env"`
	Cases      []CaseReport      `json:"cases"`
	Summary    Summary           `json:"summary"`
}

type Summary struct {
	Cases         int `json:"cases"`
	CasesPassed   int `json:"cases_passed"`
	CasesFailed   int `json:"cases_failed"`
	Queries       int `json:"queries"`
	QueriesPassed int `json:"queries_passed"`
	QueriesFailed int `json:"queries_failed"`
	Skipped       int `json:"skipped"`
}

// Finalize computes the summary counts.
func (r *Report) Finalize() {
	r.Summary = Summary{}
	for _, c := range r.Cases {
		r.Summary.Cases++
		if c.Status == StatusFail {
			r.Summary.CasesFailed++
		} else if c.Status == StatusPass {
			r.Summary.CasesPassed++
		}
		for _, q := range c.Queries {
			r.Summary.Queries++
			switch q.Status {
			case StatusPass:
				r.Summary.QueriesPassed++
			case StatusFail:
				r.Summary.QueriesFailed++
			case StatusSkip:
				r.Summary.Skipped++
			}
		}
	}
}

// WriteFiles writes e2e-report.json and e2e-report.md into dir.
func (r *Report) WriteFiles(dir string) (jsonPath, mdPath string, err error) {
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	jsonPath = filepath.Join(dir, "e2e-report.json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err = os.WriteFile(jsonPath, b, 0o644); err != nil {
		return "", "", err
	}
	mdPath = filepath.Join(dir, "e2e-report.md")
	if err = os.WriteFile(mdPath, []byte(r.markdown()), 0o644); err != nil {
		return "", "", err
	}
	return jsonPath, mdPath, nil
}

func (r *Report) markdown() string {
	var b strings.Builder
	b.WriteString("# ndc-elasticsearch e2e report\n\n")
	b.WriteString(fmt.Sprintf("- Started: `%s`\n- Finished: `%s`\n", r.StartedAt, r.FinishedAt))
	b.WriteString(fmt.Sprintf("- Cases: **%d** (%d passed, %d failed)\n",
		r.Summary.Cases, r.Summary.CasesPassed, r.Summary.CasesFailed))
	b.WriteString(fmt.Sprintf("- Queries: **%d** (%d passed, %d failed, %d skipped)\n\n",
		r.Summary.Queries, r.Summary.QueriesPassed, r.Summary.QueriesFailed, r.Summary.Skipped))

	if len(r.Env) > 0 {
		b.WriteString("Run config: ")
		first := true
		for k, v := range r.Env {
			if !first {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("`%s=%s`", k, v))
			first = false
		}
		b.WriteString("\n\n")
	}

	for _, c := range r.Cases {
		b.WriteString(fmt.Sprintf("## %s %s\n\n", statusEmoji(c.Status), c.Name))
		if c.Message != "" {
			b.WriteString(fmt.Sprintf("> %s\n\n", c.Message))
		}
		b.WriteString(fmt.Sprintf("**L3 schema conformance:** %s %s\n\n", statusEmoji(c.SchemaStatus), c.SchemaStatus))
		if len(c.SchemaProblems) > 0 {
			b.WriteString("<details><summary>schema problems</summary>\n\n")
			for _, p := range c.SchemaProblems {
				b.WriteString("- " + escapeMD(p) + "\n")
			}
			b.WriteString("\n</details>\n\n")
		}

		b.WriteString("| query | layer | target | status | LLM verdict |\n")
		b.WriteString("|---|---|---|---|---|\n")
		for _, q := range c.Queries {
			verdict := "—"
			for _, v := range q.Verdicts {
				if v.Comparison == "ddn-vs-es" {
					if v.Error != "" {
						verdict = "error: " + oneLine(v.Error)
					} else if v.Verdict != nil {
						verdict = fmt.Sprintf("equivalent=%v", v.Verdict.Equivalent)
					}
				}
			}
			b.WriteString(fmt.Sprintf("| %s | %s | `%s` | %s %s | %s |\n",
				q.Name, q.Layer, q.Target, statusEmoji(q.Status), q.Status, oneLine(verdict)))
		}
		b.WriteString("\n")

		// Failure detail blocks.
		for _, q := range c.Queries {
			if q.Status != StatusFail {
				continue
			}
			b.WriteString(fmt.Sprintf("### ❌ %s / %s\n\n", c.Name, q.Name))
			if q.Message != "" {
				b.WriteString("> " + escapeMD(q.Message) + "\n\n")
			}
			for _, v := range q.Verdicts {
				if v.Verdict != nil {
					b.WriteString(fmt.Sprintf("- **%s**: equivalent=%v — %s\n", v.Comparison, v.Verdict.Equivalent, escapeMD(v.Verdict.Rationale)))
					for _, d := range v.Verdict.Diffs {
						b.WriteString("  - " + escapeMD(d) + "\n")
					}
				} else if v.Error != "" {
					b.WriteString(fmt.Sprintf("- **%s**: error — %s\n", v.Comparison, escapeMD(v.Error)))
				}
			}
			writeDetails(&b, "DDN payload", q.DDNPayload)
			writeDetails(&b, "ES payload", q.ESPayload)
			writeDetails(&b, "golden.ddn.json", q.GoldenDDN)
			writeDetails(&b, "golden.es.json", q.GoldenES)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func writeDetails(b *strings.Builder, title, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	b.WriteString(fmt.Sprintf("<details><summary>%s</summary>\n\n```json\n%s\n```\n\n</details>\n\n", title, tail(content, 4000)))
}

func statusEmoji(s string) string {
	switch s {
	case StatusPass:
		return "✅"
	case StatusFail:
		return "❌"
	case StatusSkip:
		return "⏭️"
	default:
		return "•"
	}
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return tail(s, 200)
}

func escapeMD(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
