// Package protocol defines the frozen JSON envelope and data transfer
// objects for the okf CLI (docs/PROTOCOL.md). It contains stable DTOs only
// and never imports engine packages.
package protocol

// Version is the frozen protocol version.
const Version = 1

// Response is the top-level JSON envelope. Success responses carry Data;
// error responses carry Error.
type Response struct {
	OK      bool         `json:"ok"`
	Command string       `json:"command"`
	Version int          `json:"version"`
	Data    any          `json:"data,omitempty"`
	Error   *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail is the error payload. Details is omitted when empty.
type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Success builds a success envelope.
func Success(command string, data any) Response {
	return Response{OK: true, Command: command, Version: Version, Data: data}
}

// Failure builds an error envelope.
func Failure(command, code, message string, details map[string]string) Response {
	if len(details) == 0 {
		details = nil
	}
	return Response{
		OK:      false,
		Command: command,
		Version: Version,
		Error:   &ErrorDetail{Code: code, Message: message, Details: details},
	}
}

// TypeCount is one entry of the status type histogram.
type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// StatusData is the "status" success payload.
type StatusData struct {
	Root          string      `json:"root"`
	BundleID      string      `json:"bundle_id"`
	DocumentCount int         `json:"document_count"`
	LinkCount     int         `json:"link_count"`
	Types         []TypeCount `json:"types"`
}

// DocumentSummary is one "list" entry.
type DocumentSummary struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// ListData is the "list" success payload.
type ListData struct {
	Count     int               `json:"count"`
	Documents []DocumentSummary `json:"documents"`
}

// SearchResultItem is one "search" hit with an integer score.
type SearchResultItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Score       int    `json:"score"`
}

// SearchData is the "search" success payload.
type SearchData struct {
	Query   string             `json:"query"`
	Count   int                `json:"count"`
	Results []SearchResultItem `json:"results"`
}

// RetrieveSource is one context source record.
type RetrieveSource struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Seed         bool   `json:"seed"`
	Score        int    `json:"score"`
	Distance     int    `json:"distance"`
	Predecessor  string `json:"predecessor"`
	Relationship string `json:"relationship"`
	Truncated    bool   `json:"truncated"`
}

// RetrieveStatistics describes the assembled context.
type RetrieveStatistics struct {
	SeedCount      int  `json:"seed_count"`
	GraphCount     int  `json:"graph_count"`
	DocumentCount  int  `json:"document_count"`
	CharacterCount int  `json:"character_count"`
	MaxCharacters  int  `json:"max_characters"`
	Truncated      bool `json:"truncated"`
}

// RetrieveData is the "retrieve" success payload.
type RetrieveData struct {
	Query      string             `json:"query"`
	Context    string             `json:"context"`
	Sources    []RetrieveSource   `json:"sources"`
	Statistics RetrieveStatistics `json:"statistics"`
}

// DocumentLink is one typed link of a document.
type DocumentLink struct {
	Target    string `json:"target"`
	Predicate string `json:"predicate"`
}

// DocumentDetail is the full document payload of "get".
type DocumentDetail struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
	Links       []DocumentLink `json:"links"`
	Body        string         `json:"body"`
}

// GetData is the "get" success payload.
type GetData struct {
	Document DocumentDetail `json:"document"`
}

// GraphNodeData is one traversal record of "graph".
type GraphNodeData struct {
	ID           string `json:"id"`
	Distance     int    `json:"distance"`
	Predecessor  string `json:"predecessor"`
	Relationship string `json:"relationship"`
}

// GraphEdgeData is one edge of "graph".
type GraphEdgeData struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Predicate string `json:"predicate"`
}

// GraphData is the "graph" success payload.
type GraphData struct {
	ID        string          `json:"id"`
	Direction string          `json:"direction"`
	MaxHops   int             `json:"max_hops"`
	Nodes     []GraphNodeData `json:"nodes"`
	Edges     []GraphEdgeData `json:"edges"`
}

// GenerateData is the "generate" success payload: the diff between the
// generated bundle and the active knowledge, by document ID.
type GenerateData struct {
	Mode        string   `json:"mode"`
	PreviewRoot string   `json:"preview_root"`
	Added       []string `json:"added"`
	Changed     []string `json:"changed"`
	Removed     []string `json:"removed"`
	Unchanged   []string `json:"unchanged"`
}

// PrepareQueryData is one prepared context entry.
type PrepareQueryData struct {
	Name      string   `json:"name"`
	File      string   `json:"file"`
	Checksum  string   `json:"checksum"`
	Sources   []string `json:"sources"`
	Truncated bool     `json:"truncated"`
}

// PrepareData is the "prepare-agent-context" success payload.
type PrepareData struct {
	Output      string             `json:"output"`
	BundleID    string             `json:"bundle_id"`
	GeneratedAt string             `json:"generated_at"`
	Manifest    string             `json:"manifest"`
	Queries     []PrepareQueryData `json:"queries"`
}

// ValidationProblemData is one validation finding.
type ValidationProblemData struct {
	Code     string `json:"code"`
	Document string `json:"document"`
	Message  string `json:"message"`
}

// ValidateData is the "validate" success payload. An invalid bundle is still
// a success envelope; the exit code carries the failure (docs/PROTOCOL.md
// section 3).
type ValidateData struct {
	Mode     string                  `json:"mode"`
	Valid    bool                    `json:"valid"`
	Errors   []ValidationProblemData `json:"errors"`
	Warnings []ValidationProblemData `json:"warnings"`
}
