package retrieval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"noli/pkg/graph"
	"noli/pkg/internal/targetlock"
)

// ManifestFileName is the manifest written next to the prepared contexts.
const ManifestFileName = "manifest.json"

// QueriesVersion is the supported noli-agent-queries.yaml schema version.
const QueriesVersion = 1

// ErrUnsafeOutput marks an output location that must not be replaced.
var ErrUnsafeOutput = errors.New("unsafe output path")

// ErrWriteBusy reports that another process is preparing the same output.
// Callers may retry after the other writer completes.
var ErrWriteBusy = targetlock.ErrBusy

// queryNamePattern keeps query names safe as filenames.
var queryNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// PreparedQuery is one validated agent query.
type PreparedQuery struct {
	Name    string
	Query   string
	Options Options
}

// queriesFile is the strict noli-agent-queries.yaml schema.
type queriesFile struct {
	Version int          `yaml:"version"`
	Queries []queryEntry `yaml:"queries"`
}

type queryEntry struct {
	Name          string   `yaml:"name"`
	Query         string   `yaml:"query"`
	Types         []string `yaml:"types"`
	ExcludeTypes  []string `yaml:"exclude_types"`
	SearchLimit   int      `yaml:"search_limit"`
	MaxHops       int      `yaml:"max_hops"`
	MaxDocuments  int      `yaml:"max_documents"`
	MaxCharacters int      `yaml:"max_characters"`
	Direction     string   `yaml:"direction"`
}

// ParseQueries strictly parses noli-agent-queries.yaml and validates every
// query name before it is ever used as a filename.
func ParseQueries(data []byte) ([]PreparedQuery, error) {
	var file queriesFile
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("parse agent queries: %w", err)
	}
	if file.Version != QueriesVersion {
		return nil, fmt.Errorf("agent queries version must be %d, got %d", QueriesVersion, file.Version)
	}
	if len(file.Queries) == 0 {
		return nil, fmt.Errorf("agent queries must define at least one query")
	}
	seen := make(map[string]struct{}, len(file.Queries))
	queries := make([]PreparedQuery, 0, len(file.Queries))
	for i, entry := range file.Queries {
		location := fmt.Sprintf("queries[%d]", i)
		if !queryNamePattern.MatchString(entry.Name) {
			return nil, fmt.Errorf("%s: name %q must match %s", location, entry.Name, queryNamePattern.String())
		}
		if _, duplicate := seen[entry.Name]; duplicate {
			return nil, fmt.Errorf("%s: duplicate query name %q", location, entry.Name)
		}
		seen[entry.Name] = struct{}{}
		if strings.TrimSpace(entry.Query) == "" {
			return nil, fmt.Errorf("%s: query text is required", location)
		}
		options, err := Options{
			SearchLimit:   entry.SearchLimit,
			MaxHops:       entry.MaxHops,
			MaxDocuments:  entry.MaxDocuments,
			MaxCharacters: entry.MaxCharacters,
			Direction:     graph.Direction(entry.Direction),
			IncludeTypes:  entry.Types,
			ExcludeTypes:  entry.ExcludeTypes,
		}.Normalized()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", location, err)
		}
		queries = append(queries, PreparedQuery{Name: entry.Name, Query: entry.Query, Options: options})
	}
	return queries, nil
}

// RetrieveFunc runs one retrieval; callers usually bind okf.Store.Retrieve.
type RetrieveFunc func(query string, options Options) (Result, error)

// PrepareOptions configures a prepared-context build.
type PrepareOptions struct {
	// Output is the directory receiving context files and the manifest. An
	// existing output is replaced only when it is empty or already holds a
	// manifest.
	Output string
	// BundleID is the source bundle checksum recorded in the manifest.
	BundleID string
	// GeneratedAt is the injected clock; it is the only time value in the
	// output.
	GeneratedAt time.Time
}

// PreparedQueryResult is one manifest entry.
type PreparedQueryResult struct {
	Name      string   `json:"name"`
	Query     string   `json:"query"`
	File      string   `json:"file"`
	Checksum  string   `json:"checksum"`
	Sources   []string `json:"sources"`
	Truncated bool     `json:"truncated"`
}

// PreparedContext reports one completed build.
type PreparedContext struct {
	Output      string
	BundleID    string
	GeneratedAt string
	Manifest    string
	Queries     []PreparedQueryResult
}

// manifestDocument is the frozen manifest.json schema.
type manifestDocument struct {
	Version     int             `json:"version"`
	GeneratedAt string          `json:"generated_at"`
	BundleID    string          `json:"bundle_id"`
	Queries     []manifestQuery `json:"queries"`
}

type manifestQuery struct {
	Name          string   `json:"name"`
	Query         string   `json:"query"`
	SearchLimit   int      `json:"search_limit"`
	MaxHops       int      `json:"max_hops"`
	MaxDocuments  int      `json:"max_documents"`
	MaxCharacters int      `json:"max_characters"`
	Direction     string   `json:"direction"`
	IncludeTypes  []string `json:"include_types"`
	ExcludeTypes  []string `json:"exclude_types"`
	File          string   `json:"file"`
	Checksum      string   `json:"checksum"`
	Sources       []string `json:"sources"`
	Truncated     bool     `json:"truncated"`
}

// Prepare runs every query through the supplied retrieval function, builds
// the output in a temporary sibling directory, and replaces the output
// atomically. All file checksums are recorded in manifest.json.
func Prepare(queries []PreparedQuery, retrieve RetrieveFunc, options PrepareOptions) (PreparedContext, error) {
	if len(queries) == 0 {
		return PreparedContext{}, fmt.Errorf("no queries to prepare")
	}
	output := strings.TrimSpace(options.Output)
	if output == "" || strings.ContainsRune(output, '\x00') {
		return PreparedContext{}, fmt.Errorf("output directory %q: %w", options.Output, ErrUnsafeOutput)
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return PreparedContext{}, fmt.Errorf("resolve output directory %q: %w", output, err)
	}
	lock, err := targetlock.Acquire(absOutput + ".lock")
	if err != nil {
		return PreparedContext{}, fmt.Errorf("prepare agent context output: %w", err)
	}
	defer func() { _ = lock.Release() }()
	if err := ensureReplaceable(absOutput); err != nil {
		return PreparedContext{}, err
	}

	generatedAt := options.GeneratedAt.UTC().Format(time.RFC3339)
	manifest := manifestDocument{
		Version:     QueriesVersion,
		GeneratedAt: generatedAt,
		BundleID:    options.BundleID,
		Queries:     []manifestQuery{},
	}
	result := PreparedContext{
		Output:      output,
		BundleID:    options.BundleID,
		GeneratedAt: generatedAt,
		Manifest:    ManifestFileName,
		Queries:     []PreparedQueryResult{},
	}

	parent := filepath.Dir(absOutput)
	temporary, err := os.MkdirTemp(parent, "."+filepath.Base(absOutput)+".preparing-*")
	if err != nil {
		return PreparedContext{}, fmt.Errorf("create temporary output: %w", err)
	}
	if err := os.Chmod(temporary, 0o755); err != nil {
		_ = os.RemoveAll(temporary)
		return PreparedContext{}, fmt.Errorf("set temporary output permissions: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(temporary)
		}
	}()

	for _, query := range queries {
		retrieved, err := retrieve(query.Query, query.Options)
		if err != nil {
			return PreparedContext{}, fmt.Errorf("query %q: %w", query.Name, err)
		}
		fileName := query.Name + ".md"
		data := []byte(retrieved.Context)
		if err := os.WriteFile(filepath.Join(temporary, fileName), data, 0o644); err != nil {
			return PreparedContext{}, fmt.Errorf("write context %q: %w", fileName, err)
		}
		checksum := "sha256:" + contentChecksum(data)
		sources := make([]string, 0, len(retrieved.Sources))
		truncated := retrieved.Statistics.Truncated
		for _, source := range retrieved.Sources {
			sources = append(sources, source.ID)
		}
		manifest.Queries = append(manifest.Queries, manifestQuery{
			Name:          query.Name,
			Query:         query.Query,
			SearchLimit:   query.Options.SearchLimit,
			MaxHops:       query.Options.MaxHops,
			MaxDocuments:  query.Options.MaxDocuments,
			MaxCharacters: query.Options.MaxCharacters,
			Direction:     string(query.Options.Direction),
			IncludeTypes:  nonNil(query.Options.IncludeTypes),
			ExcludeTypes:  nonNil(query.Options.ExcludeTypes),
			File:          fileName,
			Checksum:      checksum,
			Sources:       sources,
			Truncated:     truncated,
		})
		result.Queries = append(result.Queries, PreparedQueryResult{
			Name:      query.Name,
			Query:     query.Query,
			File:      fileName,
			Checksum:  checksum,
			Sources:   sources,
			Truncated: truncated,
		})
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return PreparedContext{}, fmt.Errorf("encode manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := os.WriteFile(filepath.Join(temporary, ManifestFileName), manifestData, 0o644); err != nil {
		return PreparedContext{}, fmt.Errorf("write manifest: %w", err)
	}

	previous := temporary + ".previous"
	outputExists := false
	if _, err := os.Stat(absOutput); err == nil {
		outputExists = true
		if err := os.Rename(absOutput, previous); err != nil {
			return PreparedContext{}, fmt.Errorf("stage output for replacement: %w", err)
		}
	}
	if err := os.Rename(temporary, absOutput); err != nil {
		if outputExists {
			if rollbackErr := os.Rename(previous, absOutput); rollbackErr != nil {
				return PreparedContext{}, fmt.Errorf(
					"activate prepared output: %v; rollback failed and backup remains at %q: %w",
					err, previous, rollbackErr)
			}
		}
		return PreparedContext{}, fmt.Errorf("activate prepared output: %w", err)
	}
	cleanup = false
	_ = os.RemoveAll(previous)
	return result, nil
}

// ensureReplaceable refuses to replace an existing directory unless it is
// empty or holds a manifest from a previous prepared build.
func ensureReplaceable(absOutput string) error {
	info, err := os.Stat(absOutput)
	if err != nil {
		return nil // does not exist yet
	}
	if !info.IsDir() {
		return fmt.Errorf("output %q is not a directory: %w", absOutput, ErrUnsafeOutput)
	}
	entries, err := os.ReadDir(absOutput)
	if err != nil {
		return fmt.Errorf("inspect output %q: %w", absOutput, err)
	}
	if len(entries) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(absOutput, ManifestFileName)); err != nil {
		return fmt.Errorf("output %q contains unrelated files; refusing to replace it: %w",
			absOutput, ErrUnsafeOutput)
	}
	return nil
}

func contentChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
