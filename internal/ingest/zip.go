package ingest

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"detection_engine/internal/config"
	"detection_engine/internal/stack"
	"detection_engine/internal/workspace"
)

var (
	ErrInvalidZip  = errors.New("invalid zip archive")
	ErrZipSlip     = errors.New("zip entry escapes destination")
	ErrZipTooLarge = errors.New("zip archive exceeds size limit")
	ErrZipSymlink  = errors.New("zip archive contains a symlink")
)

func ExtractZip(src io.ReaderAt, size int64, dest string, cfg config.Config, originalName string) (stack.Source, error) {
	reader, err := zip.NewReader(src, size)
	if err != nil {
		return stack.Source{}, ErrInvalidZip
	}
	if len(reader.File) > cfg.MaxZipFiles {
		return stack.Source{}, ErrZipTooLarge
	}

	var uncompressed int64
	for _, file := range reader.File {
		if file.UncompressedSize64 > uint64(cfg.MaxUncompressed) {
			return stack.Source{}, ErrZipTooLarge
		}
		uncompressed += int64(file.UncompressedSize64)
		if uncompressed > cfg.MaxUncompressed {
			return stack.Source{}, ErrZipTooLarge
		}
		if isSymlink(file) {
			return stack.Source{}, ErrZipSymlink
		}
	}

	for _, file := range reader.File {
		if err := extractFile(file, dest); err != nil {
			return stack.Source{}, err
		}
	}

	if err := stripSingleRoot(dest); err != nil {
		return stack.Source{}, err
	}

	return stack.Source{
		Type: stack.SourceZip,
		Name: filepath.Base(originalName),
	}, nil
}

func extractFile(file *zip.File, dest string) error {
	name := strings.ReplaceAll(file.Name, "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") {
		return ErrZipSlip
	}
	cleaned := filepath.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return ErrZipSlip
	}

	target := filepath.Join(dest, cleaned)
	if _, err := workspace.Inside(dest, target); err != nil {
		return ErrZipSlip
	}

	mode := file.Mode()
	if file.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
		return os.MkdirAll(target, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("open zip entry: %w", err)
	}
	defer src.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()&^0o022|0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return err
	}
	return nil
}

func isSymlink(file *zip.File) bool {
	return file.Mode()&os.ModeSymlink != 0
}

func stripSingleRoot(dest string) error {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}
	root := filepath.Join(dest, entries[0].Name())
	tmp := dest + ".unwrapped"
	if err := os.Rename(root, tmp); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
