package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Space struct {
	ID   string
	Root string
	Repo string
}

func New(base string) (*Space, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(base, "stack-detect-"+id)
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return &Space{ID: id, Root: root, Repo: repo}, nil
}

func (s *Space) Close() error {
	if s == nil || s.Root == "" {
		return nil
	}
	return os.RemoveAll(s.Root)
}

func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func Inside(root, target string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return absTarget, nil
}
