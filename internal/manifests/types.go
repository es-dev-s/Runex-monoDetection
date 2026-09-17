package manifests

import "strings"

type Set struct {
	Node     []NodeManifest
	Python   []PythonManifest
	Go       []GoManifest
	Rust     []RustManifest
	PHP      []PHPManifest
	Ruby     []RubyManifest
	Java     []JavaManifest
	Docker   []DockerManifest
	Make     []MakeManifest
	Env      []EnvManifest
	Lock     []Lockfile
}

type NodeManifest struct {
	Path         string
	Dir          string
	Name         string
	Dependencies map[string]string
	DevDeps      map[string]string
	Scripts      map[string]string
	Engines      map[string]string
	PackageMgr   string
	Workspaces   []string
	Main         string
	Type         string
}

type PythonManifest struct {
	Path         string
	Dir          string
	Kind         string
	Requires     string
	Dependencies map[string]string
}

type GoManifest struct {
	Path         string
	Dir          string
	Module       string
	GoVersion    string
	Requirements map[string]string
}

type RustManifest struct {
	Path         string
	Dir          string
	Name         string
	Dependencies map[string]string
}

type PHPManifest struct {
	Path         string
	Dir          string
	Name         string
	Dependencies map[string]string
}

type RubyManifest struct {
	Path         string
	Dir          string
	Dependencies map[string]string
}

type JavaManifest struct {
	Path         string
	Dir          string
	Kind         string
	Dependencies []string
}

type DockerManifest struct {
	Path       string
	Dir        string
	From       string
	Image      string
	Expose     []int
	Entrypoint string
	Cmd        string
	Env        map[string]string
	BuildCmd   string
	Package    string
	Binary     string
}

type MakeManifest struct {
	Path    string
	Dir     string
	Build   string
	Run     string
	Binary  string
	Package string
}

type EnvManifest struct {
	Path string
	Dir  string
	Vars map[string]string
}

type Lockfile struct {
	Path     string
	Dir      string
	Manager  string
	Versions map[string]string
}

func (s *Set) NodeInDir(dir string) (NodeManifest, bool) {
	for _, m := range s.Node {
		if m.Dir == dir {
			return m, true
		}
	}
	return NodeManifest{}, false
}

// NodeOwnsDir is true when dir is a Node package root or nested under one.
func (s *Set) NodeOwnsDir(dir string) bool {
	for _, m := range s.Node {
		if m.Dir == dir || m.Dir == "" {
			return true
		}
		if dir != "" && strings.HasPrefix(dir, m.Dir+"/") {
			return true
		}
	}
	return false
}

func (m NodeManifest) Dep(name string) (string, bool) {
	if v, ok := m.Dependencies[name]; ok {
		return v, true
	}
	if v, ok := m.DevDeps[name]; ok {
		return v, true
	}
	return "", false
}

func NormalizeVersion(raw string) string {
	if raw == "" || raw == "*" || raw == "latest" || raw == "workspace:*" || raw == "workspace:^" {
		return ""
	}
	out := raw
	for _, prefix := range []string{"workspace:", "^", "~", ">=", "<=", ">", "<", "="} {
		if len(out) >= len(prefix) && out[:len(prefix)] == prefix {
			out = out[len(prefix):]
		}
	}
	for i, r := range out {
		if r == ' ' || r == ',' || r == '|' || r == '&' {
			out = out[:i]
			break
		}
	}
	return out
}
