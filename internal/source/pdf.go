package source

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const defaultPDFOutputLimit int64 = 32 << 20

type PDFAdapter struct {
	Command        string
	MaxOutputBytes int64
}

func (a PDFAdapter) Available() (string, error) {
	command := strings.TrimSpace(a.Command)
	if command == "" {
		command = "pdftotext"
	}
	resolved, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("PDF support requires pdftotext; command %q was not found: %w", command, err)
	}
	return resolved, nil
}

func (a PDFAdapter) Load(ctx context.Context, file File) (SourceDocument, error) {
	command, err := a.Available()
	if err != nil {
		return SourceDocument{}, fmt.Errorf("load PDF source %s: %w", file.Path, err)
	}
	limit := a.MaxOutputBytes
	if limit <= 0 {
		limit = defaultPDFOutputLimit
	}
	stdout := &boundedCapture{maximum: limit}
	stderr := &boundedCapture{maximum: 4096}
	cmd := exec.CommandContext(ctx, command, "-enc", "UTF-8", "-layout", file.Path, "-")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return SourceDocument{}, fmt.Errorf("load PDF source %s: run pdftotext: %w", file.Path, ctxErr)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return SourceDocument{}, fmt.Errorf("load PDF source %s: run pdftotext: %w: %s", file.Path, err, detail)
		}
		return SourceDocument{}, fmt.Errorf("load PDF source %s: run pdftotext: %w", file.Path, err)
	}
	if stdout.truncated {
		return SourceDocument{}, fmt.Errorf("load PDF source %s: extracted text exceeds %d bytes", file.Path, limit)
	}
	content, sections := pdfPages(stdout.String())
	return newDocument(file, content, sections, map[string]any{"pdf_extractor": "pdftotext"}), nil
}

func pdfPages(raw string) (string, []SourceSection) {
	raw = strings.TrimPrefix(raw, "\ufeff")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	pages := strings.Split(raw, "\f")
	var sections []SourceSection
	var contents []string
	currentLine := 1
	for i, page := range pages {
		page = strings.TrimSpace(page)
		if page == "" {
			continue
		}
		lineCount := len(sourceLines(page))
		sections = append(sections, SourceSection{
			Heading:   fmt.Sprintf("Page %d", i+1),
			Content:   page,
			Page:      i + 1,
			StartLine: currentLine,
			EndLine:   currentLine + lineCount - 1,
		})
		contents = append(contents, page)
		currentLine += lineCount + 1
	}
	return strings.Join(contents, "\n\n"), sections
}

type boundedCapture struct {
	data      []byte
	maximum   int64
	truncated bool
}

func (b *boundedCapture) Write(data []byte) (int, error) {
	remaining := b.maximum - int64(len(b.data))
	if remaining > 0 {
		keep := int64(len(data))
		if keep > remaining {
			keep = remaining
		}
		b.data = append(b.data, data[:keep]...)
	}
	if int64(len(data)) > remaining {
		b.truncated = true
	}
	return len(data), nil
}

func (b *boundedCapture) String() string {
	value := string(b.data)
	if b.truncated {
		return value + " …[truncated]"
	}
	return value
}
