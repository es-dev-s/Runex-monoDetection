package ingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"detection_engine/internal/config"
	"detection_engine/internal/stack"
)

var (
	ErrInvalidGitHubURL = errors.New("invalid GitHub repository")
	ErrCloneFailed      = errors.New("git clone failed")
	ErrRepoTooLarge     = errors.New("repository exceeds size limit")
	ErrGitMissing       = errors.New("git is not installed")
)

type RepoRef struct {
	Owner string
	Name  string
	Ref   string
	HTTPS string
}

func ParseGitHub(raw, explicitRef string) (RepoRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RepoRef{}, ErrInvalidGitHubURL
	}

	if strings.HasPrefix(raw, "git@github.com:") {
		rest := strings.TrimPrefix(raw, "git@github.com:")
		rest = strings.TrimSuffix(rest, ".git")
		return parseOwnerRepo(rest, explicitRef)
	}

	if !strings.Contains(raw, "://") && !strings.Contains(raw, "github.com") {
		return parseOwnerRepo(raw, explicitRef)
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return RepoRef{}, ErrInvalidGitHubURL
	}
	if parsed.User != nil {
		return RepoRef{}, ErrInvalidGitHubURL
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return RepoRef{}, ErrInvalidGitHubURL
	}

	parts := strings.FieldsFunc(parsed.Path, func(r rune) bool { return r == '/' })
	if len(parts) < 2 {
		return RepoRef{}, ErrInvalidGitHubURL
	}
	ref := explicitRef
	if ref == "" && len(parts) >= 4 && (parts[2] == "tree" || parts[2] == "commit") {
		ref = strings.Join(parts[3:], "/")
	}
	return parseOwnerRepo(parts[0]+"/"+parts[1], ref)
}

func parseOwnerRepo(value, ref string) (RepoRef, error) {
	value = strings.Trim(value, "/")
	value = strings.TrimSuffix(value, ".git")
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return RepoRef{}, ErrInvalidGitHubURL
	}
	owner, name := parts[0], parts[1]
	if !safeGitName(owner) || !safeGitName(name) {
		return RepoRef{}, ErrInvalidGitHubURL
	}
	if ref != "" && !safeRef(ref) {
		return RepoRef{}, ErrInvalidGitHubURL
	}
	return RepoRef{
		Owner: owner,
		Name:  name,
		Ref:   ref,
		HTTPS: "https://github.com/" + owner + "/" + name + ".git",
	}, nil
}

func safeGitName(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func safeRef(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("./_-", r) {
			continue
		}
		return false
	}
	return true
}

func CloneGitHub(ctx context.Context, cfg config.Config, dest string, ref RepoRef, token string) (stack.Source, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return stack.Source{}, ErrGitMissing
	}

	cloneURL := ref.HTTPS
	if token != "" {
		cloneURL = "https://x-access-token:" + token + "@github.com/" + ref.Owner + "/" + ref.Name + ".git"
	}

	args := []string{
		"-c", "core.askPass=",
		"-c", "credential.helper=",
		"clone",
		"--depth", "1",
		"--single-branch",
		"--no-tags",
	}
	if ref.Ref != "" {
		args = append(args, "--branch", ref.Ref)
	}
	args = append(args, "--", cloneURL, dest)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GCM_INTERACTIVE=never",
	}
	var stderr bytes.Buffer
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stack.Source{}, fmt.Errorf("%w: %s", ErrCloneFailed, sanitizeCloneErr(stderr.String(), token))
	}

	if err := EnforceDirSize(dest, cfg.MaxCloneBytes); err != nil {
		return stack.Source{}, err
	}

	commit := readCommit(ctx, dest)
	return stack.Source{
		Type:   stack.SourceGitHub,
		URL:    "https://github.com/" + ref.Owner + "/" + ref.Name,
		Ref:    ref.Ref,
		Commit: commit,
		Name:   path.Join(ref.Owner, ref.Name),
	}, nil
}

func readCommit(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_TERMINAL_PROMPT=0"}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sanitizeCloneErr(raw, token string) string {
	msg := strings.TrimSpace(raw)
	if token != "" {
		msg = strings.ReplaceAll(msg, token, "***")
	}
	if len(msg) > 400 {
		msg = msg[:400]
	}
	if msg == "" {
		return "clone failed"
	}
	return msg
}

func EnforceDirSize(root string, limit int64) error {
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		if total > limit {
			return ErrRepoTooLarge
		}
		return nil
	})
	return err
}
