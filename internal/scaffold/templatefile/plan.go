package templatefile

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Template paths use slash separators, independently of the host OS.
type Template struct {
	Source, Destination string
	Mode                fs.FileMode
}

type Blueprint struct {
	Directories []string
	Templates   []Template
}

type plannedFile struct {
	name    string
	content []byte
	mode    fs.FileMode
}

// Plan contains fully rendered files in explicit, deterministic order. Compile
// cannot touch the destination filesystem, and Apply never overwrites a file.
type Plan struct {
	directories []string
	files       []plannedFile
}

func (b Blueprint) Compile(sources fs.FS, data any) (*Plan, error) {
	plan := &Plan{directories: append([]string(nil), b.Directories...)}
	for _, dir := range plan.directories {
		if !safePath(dir) {
			return nil, fmt.Errorf("invalid generated directory %q", dir)
		}
	}
	seen := make(map[string]bool)
	for _, entry := range b.Templates {
		if !safePath(entry.Destination) || seen[entry.Destination] {
			return nil, fmt.Errorf("invalid or duplicate generated file %q", entry.Destination)
		}
		seen[entry.Destination] = true
		if entry.Mode == 0 || entry.Mode != entry.Mode.Perm() {
			return nil, fmt.Errorf("invalid permissions for %s", entry.Destination)
		}
		source, err := fs.ReadFile(sources, entry.Source)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", entry.Source, err)
		}
		content, err := render(entry.Destination, string(source), data)
		if err != nil {
			return nil, err
		}
		plan.files = append(plan.files, plannedFile{name: entry.Destination, content: content, mode: entry.Mode})
	}
	return plan, nil
}

func safePath(name string) bool {
	return fs.ValidPath(name) && name != "." && !strings.ContainsAny(name, "\\:")
}

func (p *Plan) Apply(ctx context.Context, root *os.Root) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, directory := range p.directories {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := root.MkdirAll(filepath.FromSlash(directory), 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", directory, err)
		}
	}
	for _, file := range p.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeContent(root, filepath.FromSlash(file.name), file.content, file.mode); err != nil {
			return err
		}
	}
	return nil
}
