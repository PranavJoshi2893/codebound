// Package templatefile writes generated files exclusively beneath an os.Root.
package templatefile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// Write renders before opening the destination and never replaces existing files.
// Root-relative operations prevent directory symlinks escaping the project.
func Write(root *os.Root, name, source string, data any, perm os.FileMode) error {
	content, err := render(name, source, data)
	if err != nil {
		return err
	}
	return writeContent(root, name, content, perm)
}

func render(name, source string, data any) ([]byte, error) {
	tmpl, err := template.New(filepath.Base(name)).Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse template for %s: %w", name, err)
	}
	var content bytes.Buffer
	if err := tmpl.Execute(&content, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return content.Bytes(), nil
}

func writeContent(root *os.Root, name string, content []byte, perm os.FileMode) error {
	if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return fmt.Errorf("create directory for %s: %w", name, err)
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write %s: %w", name, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", name, closeErr)
	}
	return nil
}
