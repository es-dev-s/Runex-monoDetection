package engine_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"detection_engine/internal/config"
	"detection_engine/internal/engine"
	"detection_engine/internal/ingest"
	"detection_engine/internal/stack"
)

func TestDetectNextJS(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "nextjs").Apps[0]
	if app.Framework == nil || app.Framework.Name != "nextjs" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.Framework.Version != "15.5.0" {
		t.Fatalf("version = %q", app.Framework.Version)
	}
	if app.Runtime == nil || app.Runtime.Name != "nodejs" {
		t.Fatalf("runtime = %+v", app.Runtime)
	}
	if app.Confidence != 100 {
		t.Fatalf("confidence = %d evidence=%+v", app.Confidence, app.Evidence)
	}
	if app.Build.Command != "next build" || app.Start.Command != "next start" || app.Port != 3000 {
		t.Fatalf("deploy profile = %+v %+v %d", app.Build, app.Start, app.Port)
	}
}

func TestDetectFastAPI(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "fastapi").Apps[0]
	if app.Framework == nil || app.Framework.Name != "fastapi" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.Framework.Version != "0.115.0" {
		t.Fatalf("version = %q", app.Framework.Version)
	}
	if app.Runtime == nil || app.Runtime.Name != "python" {
		t.Fatalf("runtime = %+v", app.Runtime)
	}
}

func TestDetectGoHTTPService(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "go-http").Apps[0]
	if app.Framework == nil || app.Framework.Name != "net/http" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.Runtime == nil || app.Runtime.Name != "go" || app.Runtime.Version != "1.26.5" {
		t.Fatalf("runtime = %+v", app.Runtime)
	}
	if app.Port != 18780 {
		t.Fatalf("port = %d", app.Port)
	}
	if !strings.Contains(app.Build.Command, "./cmd/server") {
		t.Fatalf("build = %q", app.Build.Command)
	}
	if app.Start.Command == "" {
		t.Fatalf("start empty")
	}
	if app.Confidence < 90 {
		t.Fatalf("confidence = %d evidence=%+v", app.Confidence, app.Evidence)
	}
	foundPG := false
	for _, db := range app.Databases {
		if db.Name == "postgresql" {
			foundPG = true
		}
	}
	if !foundPG {
		t.Fatalf("expected postgresql from pgx, got %+v", app.Databases)
	}
}

func TestDetectChi(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "go-chi").Apps[0]
	if app.Framework == nil || app.Framework.Name != "chi" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.Runtime == nil || app.Runtime.Name != "go" {
		t.Fatalf("runtime = %+v", app.Runtime)
	}
}

func TestDetectPythonWorker(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "python-worker").Apps[0]
	if app.Framework == nil || app.Framework.Name != "python-worker" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.Start.Command != "python -u main.py" {
		t.Fatalf("start = %q", app.Start.Command)
	}
	names := map[string]bool{}
	for _, db := range app.Databases {
		names[db.Name] = true
	}
	if !names["postgresql"] || !names["redis"] {
		t.Fatalf("databases = %+v", app.Databases)
	}
}

func TestDetectViteReactNotSvelteKit(t *testing.T) {
	t.Parallel()
	result := detectFixture(t, "vite-react")
	if len(result.Apps) != 1 {
		t.Fatalf("nested source JS must not become apps: %+v", result.Apps)
	}
	app := result.Apps[0]
	if app.Framework == nil || app.Framework.Name != "react" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.BuildTool == nil || app.BuildTool.Name != "vite" {
		t.Fatalf("build tool = %+v", app.BuildTool)
	}
}

func TestDetectTauriDesktop(t *testing.T) {
	t.Parallel()
	result := detectFixture(t, "tauri-desktop")
	found := map[string]string{}
	for _, app := range result.Apps {
		if app.Framework != nil {
			found[app.Path] = app.Framework.Name
		}
	}
	if found["."] != "nextjs" {
		t.Fatalf("web = %q apps=%+v", found["."], result.Apps)
	}
	if found["src-tauri"] != "tauri" {
		t.Fatalf("desktop = %q apps=%+v", found["src-tauri"], result.Apps)
	}
}

func TestDetectHTMLSite(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "html-site").Apps[0]
	if app.Framework == nil || app.Framework.Name != "html" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.Start.Command == "" {
		t.Fatal("expected static start command")
	}
}

func TestDetectJSScript(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "js-script").Apps[0]
	if app.Framework == nil || app.Framework.Name != "javascript" {
		t.Fatalf("framework = %+v", app.Framework)
	}
}

func TestDetectNodeScript(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "node-script").Apps[0]
	if app.Framework == nil || app.Framework.Name != "nodejs" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.Start.Command != "node index.js" {
		t.Fatalf("start = %q", app.Start.Command)
	}
}

func TestDetectMixedHTMLAndExpress(t *testing.T) {
	t.Parallel()
	result := detectFixture(t, "mixed-stack")
	if len(result.Apps) < 2 {
		t.Fatalf("apps = %+v", result.Apps)
	}
	found := map[string]string{}
	for _, app := range result.Apps {
		if app.Framework != nil {
			found[app.Path] = app.Framework.Name
		}
	}
	if found["site"] != "html" {
		t.Fatalf("site = %q apps=%+v", found["site"], result.Apps)
	}
	if found["api"] != "express" {
		t.Fatalf("api = %q apps=%+v", found["api"], result.Apps)
	}
}

func TestDetectAxum(t *testing.T) {
	t.Parallel()
	app := detectFixture(t, "axum").Apps[0]
	if app.Framework == nil || app.Framework.Name != "axum" {
		t.Fatalf("framework = %+v", app.Framework)
	}
	if app.Runtime == nil || app.Runtime.Name != "rust" {
		t.Fatalf("runtime = %+v", app.Runtime)
	}
}

func TestUnknownHasNoFramework(t *testing.T) {
	t.Parallel()
	result := detectFixture(t, "unknown")
	if len(result.Apps) == 0 {
		t.Fatal("expected language-only app")
	}
	if result.Apps[0].Framework != nil {
		t.Fatalf("unexpected framework %+v", result.Apps[0].Framework)
	}
	if result.Apps[0].Confidence >= 75 {
		t.Fatalf("overconfident on unknown repo: %d", result.Apps[0].Confidence)
	}
}

func TestConcurrentDetectHasNoRace(t *testing.T) {
	t.Parallel()
	eng := engine.New(testConfig())
	dir := fixtureDir(t, "nextjs")
	var wg sync.WaitGroup
	errCh := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := eng.DetectDir(context.Background(), "race", dir, stack.Source{Type: stack.SourceDir})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestZipSlipRejected(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "nope"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	_, err = ingest.ExtractZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()), dest, testConfig(), "evil.zip")
	if err == nil {
		t.Fatal("expected zip-slip rejection")
	}
	if _, statErr := os.Stat(filepath.Join(dest, "..", "evil.txt")); statErr == nil {
		t.Fatal("zip slip wrote outside destination")
	}
}

func TestParseGitHub(t *testing.T) {
	t.Parallel()
	ref, err := ingest.ParseGitHub("https://github.com/vercel/next.js/tree/canary", "")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Owner != "vercel" || ref.Name != "next.js" || ref.Ref != "canary" {
		t.Fatalf("%+v", ref)
	}
	if _, err := ingest.ParseGitHub("https://evil.com/vercel/next.js", ""); err == nil {
		t.Fatal("expected host rejection")
	}
	if _, err := ingest.ParseGitHub("https://user:pass@github.com/vercel/next.js", ""); err == nil {
		t.Fatal("expected credential rejection")
	}
}

func detectFixture(t *testing.T, name string) *stack.Result {
	t.Helper()
	result, err := engine.New(testConfig()).DetectDir(
		context.Background(),
		"test",
		fixtureDir(t, name),
		stack.Source{Type: stack.SourceDir, Name: name},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Apps) == 0 {
		t.Fatal("no apps")
	}
	return result
}

func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func testConfig() config.Config {
	cfg := config.Load()
	cfg.MaxConcurrent = 32
	cfg.WorkDir = os.TempDir()
	return cfg
}
