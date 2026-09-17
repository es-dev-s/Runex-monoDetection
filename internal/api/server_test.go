package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"detection_engine/internal/api"
	"detection_engine/internal/config"
	"detection_engine/internal/stack"
)

func TestDetectZipHTTP(t *testing.T) {
	t.Parallel()
	cfg := config.Load()
	cfg.WorkDir = t.TempDir()
	server := httptest.NewServer(api.New(cfg, nil).Handler())
	t.Cleanup(server.Close)

	zipPath := writeFixtureZip(t)
	file, err := os.Open(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "nextjs.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, file); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(server.URL+"/v1/detect/zip", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	var result stack.Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Apps) == 0 || result.Apps[0].Framework == nil || result.Apps[0].Framework.Name != "nextjs" {
		t.Fatalf("result = %+v", result)
	}
}

func TestRejectsNonGitHubHost(t *testing.T) {
	t.Parallel()
	cfg := config.Load()
	cfg.WorkDir = t.TempDir()
	server := httptest.NewServer(api.New(cfg, nil).Handler())
	t.Cleanup(server.Close)

	resp, err := http.Post(server.URL+"/v1/detect/github", "application/json", bytes.NewBufferString(`{"url":"https://evil.example/a/b"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func writeFixtureZip(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "nextjs"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "nextjs.zip")
	file, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}
