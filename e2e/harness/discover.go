//go:build e2e

package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	yaml "gopkg.in/yaml.v3"
)

// Case is a single discovered e2e test case (one directory under e2e/cases).
type Case struct {
	Name string // directory name, e.g. "kibana_sample_logs"
	Dir  string // absolute path

	// Seed inputs (all optional, applied in order into the fresh ES).
	IndexMappings []string // indices/*.mapping.json  (sorted)
	DataFiles     []string // data/*.ndjson           (sorted)
	InitScripts   []string // init/*.sh, init/*.http  (sorted)

	Meta    CaseMeta // parsed case.yaml (zero value if absent)
	Queries []Query  // discovered under queries/
}

// CaseMeta is the parsed case.yaml.
type CaseMeta struct {
	// KibanaSample loads an elastic.co sample dataset via the Kibana sample-data
	// API. One of: logs | ecommerce | flights. Empty => no sample data.
	KibanaSample string `yaml:"kibana_sample"`

	// Description is a human-readable note shown in the report.
	Description string `yaml:"description"`
}

// Query is a single discovered query under queries/<name>.
type Query struct {
	Name string // directory name
	Dir  string // absolute path

	GraphQL       string // query.graphql (required)
	Variables     []byte // variables.json (optional, raw JSON)
	ESSearch      []byte // es_search.json (required unless UpdateGolden regenerates)
	Target        Target // target.yaml (required)
	GoldenDDNPath string // absolute path to golden.ddn.json
	GoldenESPath  string // absolute path to golden.es.json
}

// Target selects where the equivalent raw ES query is sent and other per-query
// knobs.
type Target struct {
	// Index is the ES index or alias used for the direct _search call and for
	// the L3 schema assertion of this query's collection. For Kibana sample
	// data this must be the data-stream alias, e.g. "kibana_sample_data_logs".
	Index string `yaml:"index"`

	// Description shown in the report.
	Description string `yaml:"description"`
}

// DiscoverCases scans e2e/cases, honoring the optional single-case filter.
func DiscoverCases(env *Env) ([]Case, error) {
	entries, err := os.ReadDir(env.CasesDir)
	if err != nil {
		return nil, fmt.Errorf("reading cases dir %s: %w", env.CasesDir, err)
	}

	var cases []Case
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if env.CaseFilter != "" && name != env.CaseFilter {
			continue
		}
		c, err := loadCase(filepath.Join(env.CasesDir, name))
		if err != nil {
			return nil, fmt.Errorf("loading case %q: %w", name, err)
		}
		cases = append(cases, *c)
	}

	sort.Slice(cases, func(i, j int) bool { return cases[i].Name < cases[j].Name })

	if env.CaseFilter != "" && len(cases) == 0 {
		return nil, fmt.Errorf("case %q not found under %s", env.CaseFilter, env.CasesDir)
	}
	return cases, nil
}

func loadCase(dir string) (*Case, error) {
	c := &Case{Name: filepath.Base(dir), Dir: dir}

	var err error
	if c.IndexMappings, err = globSorted(dir, "indices", "*.mapping.json"); err != nil {
		return nil, err
	}
	if c.DataFiles, err = globSorted(dir, "data", "*.ndjson"); err != nil {
		return nil, err
	}
	initSh, err := globSorted(dir, "init", "*.sh")
	if err != nil {
		return nil, err
	}
	initHTTP, err := globSorted(dir, "init", "*.http")
	if err != nil {
		return nil, err
	}
	c.InitScripts = append(append([]string{}, initSh...), initHTTP...)
	sort.Strings(c.InitScripts)

	// case.yaml (optional)
	metaPath := filepath.Join(dir, "case.yaml")
	if b, err := os.ReadFile(metaPath); err == nil {
		if err := yaml.Unmarshal(b, &c.Meta); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", metaPath, err)
		}
	}

	// queries/<name>
	queriesDir := filepath.Join(dir, "queries")
	qEntries, err := os.ReadDir(queriesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("case %q has no queries/ directory", c.Name)
		}
		return nil, err
	}
	for _, qe := range qEntries {
		if !qe.IsDir() {
			continue
		}
		q, err := loadQuery(filepath.Join(queriesDir, qe.Name()))
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", qe.Name(), err)
		}
		c.Queries = append(c.Queries, *q)
	}
	sort.Slice(c.Queries, func(i, j int) bool { return c.Queries[i].Name < c.Queries[j].Name })

	if len(c.Queries) == 0 {
		return nil, fmt.Errorf("case %q has no queries under queries/", c.Name)
	}
	return c, nil
}

func loadQuery(dir string) (*Query, error) {
	q := &Query{
		Name:          filepath.Base(dir),
		Dir:           dir,
		GoldenDDNPath: filepath.Join(dir, "golden.ddn.json"),
		GoldenESPath:  filepath.Join(dir, "golden.es.json"),
	}

	gql, err := os.ReadFile(filepath.Join(dir, "query.graphql"))
	if err != nil {
		return nil, fmt.Errorf("query.graphql is required: %w", err)
	}
	q.GraphQL = string(gql)

	if b, err := os.ReadFile(filepath.Join(dir, "variables.json")); err == nil {
		q.Variables = b
	}

	if b, err := os.ReadFile(filepath.Join(dir, "es_search.json")); err == nil {
		q.ESSearch = b
	} else {
		return nil, fmt.Errorf("es_search.json is required: %w", err)
	}

	tb, err := os.ReadFile(filepath.Join(dir, "target.yaml"))
	if err != nil {
		return nil, fmt.Errorf("target.yaml is required: %w", err)
	}
	if err := yaml.Unmarshal(tb, &q.Target); err != nil {
		return nil, fmt.Errorf("parsing target.yaml: %w", err)
	}
	if q.Target.Index == "" {
		return nil, fmt.Errorf("target.yaml must set `index`")
	}
	return q, nil
}

// globSorted returns sorted absolute matches for <dir>/<sub>/<pattern>, or an
// empty slice if the subdirectory does not exist.
func globSorted(dir, sub, pattern string) ([]string, error) {
	subDir := filepath.Join(dir, sub)
	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		return nil, nil
	}
	matches, err := filepath.Glob(filepath.Join(subDir, pattern))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}
