package manifests

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"golang.org/x/mod/modfile"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"detection_engine/internal/config"
	"detection_engine/internal/index"
)

type parseResult struct {
	node   *NodeManifest
	python *PythonManifest
	golang *GoManifest
	rust   *RustManifest
	php    *PHPManifest
	ruby   *RubyManifest
	java   *JavaManifest
	docker *DockerManifest
	make   *MakeManifest
	env    *EnvManifest
	lock   *Lockfile
}

func Parse(ctx context.Context, idx *index.Index, cfg config.Config) (*Set, error) {
	set := &Set{}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.ManifestWorkers)
	out := make(chan parseResult, cfg.ManifestWorkers*2)

	g.Go(func() error {
		defer close(out)
		inner, innerCtx := errgroup.WithContext(ctx)
		inner.SetLimit(cfg.ManifestWorkers)
		for i := range idx.Files {
			file := idx.Files[i]
			name := strings.ToLower(file.Name)
			if !interesting(name) {
				continue
			}
			inner.Go(func() error {
				if err := innerCtx.Err(); err != nil {
					return err
				}
				data, err := index.ReadAbs(file.Abs, cfg.MaxFileReadBytes)
				if err != nil {
					return nil
				}
				res := parseFile(file, name, data)
				select {
				case out <- res:
					return nil
				case <-innerCtx.Done():
					return innerCtx.Err()
				}
			})
		}
		return inner.Wait()
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for res := range out {
			if res.node != nil {
				set.Node = append(set.Node, *res.node)
			}
			if res.python != nil {
				set.Python = append(set.Python, *res.python)
			}
			if res.golang != nil {
				set.Go = append(set.Go, *res.golang)
			}
			if res.rust != nil {
				set.Rust = append(set.Rust, *res.rust)
			}
			if res.php != nil {
				set.PHP = append(set.PHP, *res.php)
			}
			if res.ruby != nil {
				set.Ruby = append(set.Ruby, *res.ruby)
			}
			if res.java != nil {
				set.Java = append(set.Java, *res.java)
			}
			if res.docker != nil {
				set.Docker = append(set.Docker, *res.docker)
			}
			if res.make != nil {
				set.Make = append(set.Make, *res.make)
			}
			if res.env != nil {
				set.Env = append(set.Env, *res.env)
			}
			if res.lock != nil {
				set.Lock = append(set.Lock, *res.lock)
			}
		}
	}()

	if err := g.Wait(); err != nil {
		return nil, err
	}
	<-done
	attachLockVersions(set)
	return set, nil
}

func interesting(name string) bool {
	switch name {
	case "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb",
		"pyproject.toml", "requirements.txt", "pipfile", "poetry.lock",
		"go.mod", "cargo.toml", "cargo.lock",
		"composer.json", "composer.lock", "gemfile", "gemfile.lock",
		"pom.xml", "build.gradle", "build.gradle.kts",
		"dockerfile", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml",
		"pnpm-workspace.yaml", "makefile", ".env", ".env.example", ".env.sample":
		return true
	default:
		return strings.HasPrefix(name, "dockerfile")
	}
}

func parseFile(file index.File, name string, data []byte) parseResult {
	dir := file.Dir
	switch name {
	case "package.json":
		if m, ok := parseNode(file.Rel, dir, data); ok {
			return parseResult{node: &m}
		}
	case "package-lock.json":
		return parseResult{lock: parseNPMLock(file.Rel, dir, data)}
	case "pnpm-lock.yaml":
		return parseResult{lock: &Lockfile{Path: file.Rel, Dir: dir, Manager: "pnpm", Versions: parsePnpmLock(data)}}
	case "yarn.lock":
		return parseResult{lock: &Lockfile{Path: file.Rel, Dir: dir, Manager: "yarn"}}
	case "bun.lock", "bun.lockb":
		return parseResult{lock: &Lockfile{Path: file.Rel, Dir: dir, Manager: "bun"}}
	case "pyproject.toml":
		if m, ok := parsePyProject(file.Rel, dir, data); ok {
			return parseResult{python: &m}
		}
	case "requirements.txt":
		if m, ok := parseRequirements(file.Rel, dir, data); ok {
			return parseResult{python: &m}
		}
	case "pipfile":
		if m, ok := parsePipfile(file.Rel, dir, data); ok {
			return parseResult{python: &m}
		}
	case "go.mod":
		if m, ok := parseGoMod(file.Rel, dir, data); ok {
			return parseResult{golang: &m}
		}
	case "cargo.toml":
		if m, ok := parseCargo(file.Rel, dir, data); ok {
			return parseResult{rust: &m}
		}
	case "composer.json":
		if m, ok := parseComposer(file.Rel, dir, data); ok {
			return parseResult{php: &m}
		}
	case "gemfile":
		if m, ok := parseGemfile(file.Rel, dir, data); ok {
			return parseResult{ruby: &m}
		}
	case "pom.xml":
		if m, ok := parsePOM(file.Rel, dir, data); ok {
			return parseResult{java: &m}
		}
	case "build.gradle", "build.gradle.kts":
		if m, ok := parseGradle(file.Rel, dir, name, data); ok {
			return parseResult{java: &m}
		}
	case "dockerfile":
		if m, ok := parseDockerfile(file.Rel, dir, data); ok {
			return parseResult{docker: &m}
		}
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		if m, ok := parseCompose(file.Rel, dir, data); ok {
			return parseResult{docker: &m}
		}
	case "makefile":
		if m, ok := parseMakefile(file.Rel, dir, data); ok {
			return parseResult{make: &m}
		}
	case ".env", ".env.example", ".env.sample":
		if m, ok := parseEnvFile(file.Rel, dir, data); ok {
			return parseResult{env: &m}
		}
	}
	if strings.HasPrefix(name, "dockerfile") {
		if m, ok := parseDockerfile(file.Rel, dir, data); ok {
			return parseResult{docker: &m}
		}
	}
	return parseResult{}
}

func parseNode(rel, dir string, data []byte) (NodeManifest, bool) {
	var raw struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
		Engines         map[string]string `json:"engines"`
		PackageManager  string            `json:"packageManager"`
		Workspaces      json.RawMessage   `json:"workspaces"`
		Main            string            `json:"main"`
		Type            string            `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return NodeManifest{}, false
	}
	if raw.Dependencies == nil {
		raw.Dependencies = map[string]string{}
	}
	if raw.DevDependencies == nil {
		raw.DevDependencies = map[string]string{}
	}
	if raw.Scripts == nil {
		raw.Scripts = map[string]string{}
	}
	return NodeManifest{
		Path:         rel,
		Dir:          dir,
		Name:         raw.Name,
		Dependencies: raw.Dependencies,
		DevDeps:      raw.DevDependencies,
		Scripts:      raw.Scripts,
		Engines:      raw.Engines,
		PackageMgr:   raw.PackageManager,
		Workspaces:   parseWorkspaces(raw.Workspaces),
		Main:         raw.Main,
		Type:         raw.Type,
	}, true
}

func parseWorkspaces(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Packages
	}
	return nil
}

func parseNPMLock(rel, dir string, data []byte) *Lockfile {
	var raw struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return &Lockfile{Path: rel, Dir: dir, Manager: "npm"}
	}
	versions := map[string]string{}
	for key, pkg := range raw.Packages {
		name := strings.TrimPrefix(key, "node_modules/")
		if name == "" || strings.Contains(name, "/node_modules/") {
			continue
		}
		if pkg.Version != "" {
			versions[name] = pkg.Version
		}
	}
	for name, pkg := range raw.Dependencies {
		if _, ok := versions[name]; !ok && pkg.Version != "" {
			versions[name] = pkg.Version
		}
	}
	return &Lockfile{Path: rel, Dir: dir, Manager: "npm", Versions: versions}
}

func parsePnpmLock(data []byte) map[string]string {
	var raw struct {
		Packages map[string]any `json:"packages"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	versions := map[string]string{}
	for key := range raw.Packages {
		key = strings.TrimPrefix(key, "/")
		if at := strings.LastIndex(key, "@"); at > 0 {
			versions[key[:at]] = strings.TrimSuffix(key[at+1:], "(")
		}
	}
	return versions
}

func parsePyProject(rel, dir string, data []byte) (PythonManifest, bool) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return PythonManifest{}, false
	}
	deps := map[string]string{}
	requires := ""
	if project, ok := raw["project"].(map[string]any); ok {
		requires, _ = project["requires-python"].(string)
		addPythonDeps(deps, project["dependencies"])
	}
	if tool, ok := raw["tool"].(map[string]any); ok {
		if poetry, ok := tool["poetry"].(map[string]any); ok {
			if pdeps, ok := poetry["dependencies"].(map[string]any); ok {
				for name, ver := range pdeps {
					if name == "python" {
						if s, ok := ver.(string); ok {
							requires = s
						}
						continue
					}
					deps[strings.ToLower(name)] = stringify(ver)
				}
			}
		}
	}
	return PythonManifest{Path: rel, Dir: dir, Kind: "pyproject", Requires: requires, Dependencies: deps}, true
}

func parseRequirements(rel, dir string, data []byte) (PythonManifest, bool) {
	deps := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		name, ver := splitPyReq(line)
		if name != "" {
			deps[name] = ver
		}
	}
	return PythonManifest{Path: rel, Dir: dir, Kind: "requirements", Dependencies: deps}, len(deps) > 0
}

func parsePipfile(rel, dir string, data []byte) (PythonManifest, bool) {
	var raw struct {
		Packages    map[string]any `toml:"packages"`
		DevPackages map[string]any `toml:"dev-packages"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return PythonManifest{}, false
	}
	deps := map[string]string{}
	for name, ver := range raw.Packages {
		deps[strings.ToLower(name)] = stringify(ver)
	}
	return PythonManifest{Path: rel, Dir: dir, Kind: "pipfile", Dependencies: deps}, true
}

func parseGoMod(rel, dir string, data []byte) (GoManifest, bool) {
	f, err := modfile.Parse(rel, data, nil)
	if err != nil {
		return GoManifest{}, false
	}
	reqs := map[string]string{}
	if f.Module == nil {
		return GoManifest{}, false
	}
	for _, r := range f.Require {
		reqs[r.Mod.Path] = r.Mod.Version
	}
	goVersion := ""
	if f.Go != nil {
		goVersion = f.Go.Version
	}
	if f.Toolchain != nil {
		if name := strings.TrimPrefix(strings.TrimSpace(f.Toolchain.Name), "go"); name != "" {
			goVersion = name
		}
	}
	return GoManifest{Path: rel, Dir: dir, Module: f.Module.Mod.Path, GoVersion: goVersion, Requirements: reqs}, true
}

func parseCargo(rel, dir string, data []byte) (RustManifest, bool) {
	var raw struct {
		Package struct {
			Name string `toml:"name"`
		} `toml:"package"`
		Dependencies map[string]any `toml:"dependencies"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return RustManifest{}, false
	}
	deps := map[string]string{}
	for name, ver := range raw.Dependencies {
		deps[name] = stringify(ver)
	}
	return RustManifest{Path: rel, Dir: dir, Name: raw.Package.Name, Dependencies: deps}, true
}

func parseComposer(rel, dir string, data []byte) (PHPManifest, bool) {
	var raw struct {
		Name       string            `json:"name"`
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return PHPManifest{}, false
	}
	deps := map[string]string{}
	for k, v := range raw.Require {
		deps[k] = v
	}
	for k, v := range raw.RequireDev {
		if _, ok := deps[k]; !ok {
			deps[k] = v
		}
	}
	return PHPManifest{Path: rel, Dir: dir, Name: raw.Name, Dependencies: deps}, true
}

var gemRe = regexp.MustCompile(`gem\s+['"]([^'"]+)['"](?:\s*,\s*['"]([^'"]+)['"])?`)

func parseGemfile(rel, dir string, data []byte) (RubyManifest, bool) {
	deps := map[string]string{}
	for _, match := range gemRe.FindAllSubmatch(data, -1) {
		deps[string(match[1])] = string(match[2])
	}
	return RubyManifest{Path: rel, Dir: dir, Dependencies: deps}, len(deps) > 0
}

func parsePOM(rel, dir string, data []byte) (JavaManifest, bool) {
	var raw struct {
		Dependencies struct {
			Dependency []struct {
				GroupID    string `xml:"groupId"`
				ArtifactID string `xml:"artifactId"`
			} `xml:"dependency"`
		} `xml:"dependencies"`
	}
	if err := xml.Unmarshal(data, &raw); err != nil {
		return JavaManifest{}, false
	}
	var deps []string
	for _, d := range raw.Dependencies.Dependency {
		deps = append(deps, d.GroupID+":"+d.ArtifactID)
	}
	return JavaManifest{Path: rel, Dir: dir, Kind: "pom", Dependencies: deps}, true
}

func parseGradle(rel, dir, name string, data []byte) (JavaManifest, bool) {
	text := string(data)
	var deps []string
	for _, needle := range []string{"org.springframework.boot", "spring-boot", "quarkus"} {
		if strings.Contains(text, needle) {
			deps = append(deps, needle)
		}
	}
	return JavaManifest{Path: rel, Dir: dir, Kind: name, Dependencies: deps}, len(deps) > 0 || strings.Contains(text, "java")
}

func parseDockerfile(rel, dir string, data []byte) (DockerManifest, bool) {
	m := DockerManifest{Path: rel, Dir: dir, Env: map[string]string{}}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "FROM":
			image := fields[len(fields)-1]
			if m.From == "" {
				m.From = image
			}
			m.Image = image
		case "EXPOSE":
			for _, part := range fields[1:] {
				port := parsePortToken(part)
				if port > 0 {
					m.Expose = append(m.Expose, port)
				}
			}
		case "ENTRYPOINT":
			m.Entrypoint = firstDockerArg(strings.TrimSpace(line[len(fields[0]):]))
		case "CMD":
			m.Cmd = dockerArgLine(strings.TrimSpace(line[len(fields[0]):]))
		case "ENV":
			for key, value := range parseEnvAssign(strings.TrimSpace(line[3:])) {
				m.Env[key] = value
			}
		case "RUN":
			if cmd, pkg, bin := parseGoBuild(line); cmd != "" {
				m.BuildCmd = strings.TrimSpace(strings.TrimPrefix(cmd, "RUN"))
				m.Package = pkg
				m.Binary = bin
			}
		}
	}
	return m, m.From != "" || m.BuildCmd != "" || m.Entrypoint != "" || len(m.Expose) > 0
}

func parseMakefile(rel, dir string, data []byte) (MakeManifest, bool) {
	m := MakeManifest{Path: rel, Dir: dir}
	target := ""
	for _, raw := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(raw, "\t") || strings.HasPrefix(raw, "    ") {
			line := strings.TrimSpace(raw)
			if cmd, pkg, bin := parseGoBuild(line); cmd != "" {
				if target == "build" || target == "all" || m.Build == "" {
					m.Build = cmd
					m.Package = pkg
					m.Binary = bin
				}
			}
			if strings.Contains(line, "go run") && (target == "run" || m.Run == "") {
				m.Run = line
			}
			continue
		}
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, ":"); i > 0 && !strings.Contains(line[:i], "=") {
			target = strings.Fields(line[:i])[0]
		}
	}
	return m, m.Build != "" || m.Run != ""
}

func parseEnvFile(rel, dir string, data []byte) (EnvManifest, bool) {
	vars := map[string]string{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for key, value := range parseEnvAssign(line) {
			vars[key] = value
		}
	}
	return EnvManifest{Path: rel, Dir: dir, Vars: vars}, len(vars) > 0
}

func parseGoBuild(line string) (cmd, pkg, bin string) {
	if !strings.Contains(line, "go build") {
		return "", "", ""
	}
	cmd = strings.TrimSpace(line)
	fields := strings.Fields(line)
	for i, field := range fields {
		if field == "-o" && i+1 < len(fields) {
			bin = fields[i+1]
		}
		if strings.HasPrefix(field, "./") || strings.HasPrefix(field, "cmd/") {
			pkg = field
		}
	}
	return cmd, pkg, bin
}

func parsePortToken(raw string) int {
	raw = strings.TrimSuffix(strings.Split(raw, "/")[0], "/")
	raw = strings.TrimPrefix(raw, ":")
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	if n < 1 || n > 65535 {
		return 0
	}
	return n
}

func parseEnvAssign(raw string) map[string]string {
	out := map[string]string{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}
	parts := strings.Fields(raw)
	if strings.Contains(parts[0], "=") {
		for _, part := range parts {
			key, value, ok := strings.Cut(part, "=")
			if ok {
				out[key] = strings.Trim(value, `"'`)
			}
		}
		return out
	}
	if len(parts) >= 2 {
		out[parts[0]] = strings.Trim(strings.Join(parts[1:], " "), `"'`)
	}
	return out
}

func firstDockerArg(raw string) string {
	if line := dockerArgLine(raw); line != "" {
		return strings.Fields(line)[0]
	}
	return ""
}

func dockerArgLine(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		var args []string
		if err := json.Unmarshal([]byte(raw), &args); err == nil && len(args) > 0 {
			return strings.Join(args, " ")
		}
	}
	return strings.Trim(raw, `"'`)
}

func parseCompose(rel, dir string, data []byte) (DockerManifest, bool) {
	return DockerManifest{Path: rel, Dir: dir, Image: "compose"}, true
}

func addPythonDeps(dest map[string]string, raw any) {
	list, ok := raw.([]any)
	if !ok {
		return
	}
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			continue
		}
		name, ver := splitPyReq(s)
		if name != "" {
			dest[name] = ver
		}
	}
}

func splitPyReq(line string) (string, string) {
	line = strings.TrimSpace(strings.Split(line, ";")[0])
	if line == "" {
		return "", ""
	}
	name, ver := line, ""
	seps := []string{"===", "==", "~=", ">=", "<=", ">", "<"}
	for _, sep := range seps {
		if i := strings.Index(line, sep); i > 0 {
			name = strings.TrimSpace(line[:i])
			ver = strings.TrimSpace(line[i+len(sep):])
			break
		}
	}
	if i := strings.IndexAny(name, "["); i > 0 {
		name = name[:i]
	}
	return strings.ToLower(strings.TrimSpace(name)), ver
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if ver, ok := t["version"].(string); ok {
			return ver
		}
	}
	return ""
}

func attachLockVersions(set *Set) {
	locks := map[string]Lockfile{}
	for _, lock := range set.Lock {
		locks[lock.Dir] = lock
	}
	for i, node := range set.Node {
		lock, ok := locks[node.Dir]
		if !ok {
			lock, ok = locks[""]
		}
		if !ok || lock.Versions == nil {
			continue
		}
		for name, declared := range node.Dependencies {
			if exact, found := lock.Versions[name]; found {
				node.Dependencies[name] = exact
			} else {
				node.Dependencies[name] = NormalizeVersion(declared)
			}
		}
		set.Node[i] = node
	}
}

func PackageManager(idx *index.Index, set *Set, dir string) (string, string) {
	if node, ok := set.NodeInDir(dir); ok && node.PackageMgr != "" {
		name, _, _ := strings.Cut(node.PackageMgr, "@")
		return name, node.Path
	}
	type hint struct {
		file    string
		manager string
	}
	hints := []hint{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lock", "bun"},
		{"bun.lockb", "bun"},
		{"package-lock.json", "npm"},
		{"poetry.lock", "poetry"},
		{"Pipfile.lock", "pipenv"},
		{"Cargo.lock", "cargo"},
		{"go.sum", "go"},
		{"composer.lock", "composer"},
		{"Gemfile.lock", "bundler"},
	}
	for _, h := range hints {
		if file, ok := idx.HasInDir(dir, h.file); ok {
			return h.manager, file.Rel
		}
	}
	for _, rust := range set.Rust {
		if rust.Dir == dir {
			return "cargo", rust.Path
		}
	}
	for _, golang := range set.Go {
		if golang.Dir == dir {
			return "go", golang.Path
		}
	}
	if dir != "" {
		for _, h := range hints {
			if file, ok := idx.HasInDir("", h.file); ok {
				return h.manager, file.Rel
			}
		}
	}
	if _, ok := set.NodeInDir(dir); ok {
		return "npm", filepath.ToSlash(filepath.Join(dir, "package.json"))
	}
	return "", ""
}
