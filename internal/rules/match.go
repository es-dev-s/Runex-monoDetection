package rules

import (
	"path/filepath"
	"strings"

	"detection_engine/internal/index"
	"detection_engine/internal/manifests"
	"detection_engine/internal/stack"
)

type Candidate struct {
	Profile  Profile
	Dir      string
	Version  string
	Evidence []stack.Evidence
	Score    int
}

func Match(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	out = append(out, matchNode(idx, set)...)
	out = append(out, matchPython(idx, set)...)
	out = append(out, matchPythonService(idx, set)...)
	out = append(out, matchGo(idx, set)...)
	out = append(out, matchGoService(idx, set)...)
	out = append(out, matchRust(idx, set)...)
	out = append(out, matchPHP(idx, set)...)
	out = append(out, matchRuby(idx, set)...)
	out = append(out, matchJava(idx, set)...)
	out = append(out, matchStatic(idx, set)...)
	return out
}

func matchNode(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	for _, man := range set.Node {
		for _, profile := range profiles {
			if profile.Ecosystem != "node" {
				continue
			}
			c := Candidate{Profile: profile, Dir: man.Dir}
			for _, dep := range profile.Deps {
				if ver, ok := man.Dep(dep); ok {
					c.add(stack.LayerManifest, stack.KindDependency, man.Path, dep+"@"+displayVer(ver), 50)
					if c.Version == "" {
						c.Version = manifests.NormalizeVersion(ver)
					}
				}
			}
			addConfigEvidence(idx, &c, man.Dir, profile)
			addScriptEvidence(&c, man, profile)
			if c.Score > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func matchPython(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	for _, man := range set.Python {
		for _, profile := range profiles {
			if profile.Ecosystem != "python" {
				continue
			}
			c := Candidate{Profile: profile, Dir: man.Dir}
			for _, dep := range profile.Deps {
				if ver, ok := man.Dependencies[dep]; ok {
					c.add(stack.LayerManifest, stack.KindDependency, man.Path, dep+"@"+displayVer(ver), 50)
					if c.Version == "" {
						c.Version = manifests.NormalizeVersion(ver)
					}
				}
			}
			addFileEvidence(idx, &c, man.Dir, profile)
			if c.Score > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func matchGo(_ *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	for _, man := range set.Go {
		for _, profile := range profiles {
			if profile.Ecosystem != "go" {
				continue
			}
			c := Candidate{Profile: profile, Dir: man.Dir}
			for _, mod := range profile.GoModules {
				if ver, ok := man.Requirements[mod]; ok {
					c.add(stack.LayerManifest, stack.KindDependency, man.Path, mod+"@"+ver, 50)
					c.Version = strings.TrimPrefix(ver, "v")
				}
			}
			if c.Score > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func matchRust(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	for _, man := range set.Rust {
		for _, profile := range profiles {
			if profile.Ecosystem != "rust" {
				continue
			}
			c := Candidate{Profile: profile, Dir: man.Dir}
			for _, dep := range profile.Deps {
				if ver, ok := man.Dependencies[dep]; ok {
					c.add(stack.LayerManifest, stack.KindDependency, man.Path, dep+"@"+displayVer(ver), 50)
					c.Version = manifests.NormalizeVersion(ver)
				}
			}
			addConfigEvidence(idx, &c, man.Dir, profile)
			addFileEvidence(idx, &c, man.Dir, profile)
			if c.Score > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func matchPHP(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	for _, man := range set.PHP {
		for _, profile := range profiles {
			if profile.Ecosystem != "php" {
				continue
			}
			c := Candidate{Profile: profile, Dir: man.Dir}
			for _, dep := range profile.Deps {
				if ver, ok := man.Dependencies[dep]; ok {
					c.add(stack.LayerManifest, stack.KindDependency, man.Path, dep+"@"+ver, 50)
					c.Version = manifests.NormalizeVersion(ver)
				}
			}
			addFileEvidence(idx, &c, man.Dir, profile)
			if c.Score > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func matchRuby(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	for _, man := range set.Ruby {
		for _, profile := range profiles {
			if profile.Ecosystem != "ruby" {
				continue
			}
			c := Candidate{Profile: profile, Dir: man.Dir}
			for _, dep := range profile.Deps {
				if ver, ok := man.Dependencies[dep]; ok {
					c.add(stack.LayerManifest, stack.KindDependency, man.Path, dep+"@"+displayVer(ver), 50)
					c.Version = manifests.NormalizeVersion(ver)
				}
			}
			addFileEvidence(idx, &c, man.Dir, profile)
			if c.Score > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func matchJava(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	for _, man := range set.Java {
		for _, profile := range profiles {
			if profile.Ecosystem != "java" {
				continue
			}
			c := Candidate{Profile: profile, Dir: man.Dir}
			for _, dep := range profile.Deps {
				for _, found := range man.Dependencies {
					if found == dep || strings.Contains(found, dep) {
						c.add(stack.LayerManifest, stack.KindDependency, man.Path, found, 50)
					}
				}
			}
			addFileEvidence(idx, &c, man.Dir, profile)
			if c.Score > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func addConfigEvidence(idx *index.Index, c *Candidate, dir string, profile Profile) {
	if file, ok := idx.HasInDir(dir, profile.Configs...); ok {
		c.add(stack.LayerRule, stack.KindConfig, file.Rel, file.Name, 25)
	}
}

func addFileEvidence(idx *index.Index, c *Candidate, dir string, profile Profile) {
	if file, ok := idx.HasInDir(dir, profile.Files...); ok {
		c.add(stack.LayerRule, stack.KindFile, file.Rel, file.Name, 25)
	}
}

func addScriptEvidence(c *Candidate, man manifests.NodeManifest, profile Profile) {
	for _, needle := range profile.Scripts {
		for _, script := range man.Scripts {
			if strings.Contains(strings.ToLower(script), needle) {
				c.add(stack.LayerRule, stack.KindScript, man.Path, "scripts: "+needle, 25)
				return
			}
		}
	}
}

func (c *Candidate) add(layer, kind, path, detail string, weight int) {
	for _, ev := range c.Evidence {
		if ev.Kind == kind && ev.Path == path && ev.Detail == detail {
			return
		}
	}
	c.Evidence = append(c.Evidence, stack.Evidence{
		Layer:  layer,
		Kind:   kind,
		Path:   path,
		Detail: detail,
		Weight: weight,
	})
	c.Score += weight
	if c.Score > 100 {
		c.Score = 100
	}
}

func displayVer(v string) string {
	if v == "" {
		return "present"
	}
	return v
}

func DetectDatabases(set *manifests.Set, dir string) []stack.Tech {
	seen := map[string]struct{}{}
	var out []stack.Tech
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, stack.Tech{Name: name})
	}
	inScope := func(manifestDir string) bool {
		return manifestDir == dir || dir == "" || manifestDir == ""
	}
	for _, man := range set.Node {
		if !inScope(man.Dir) {
			continue
		}
		for dep := range man.Dependencies {
			add(databases[dep])
		}
		for dep := range man.DevDeps {
			add(databases[dep])
		}
	}
	for _, man := range set.Python {
		if !inScope(man.Dir) {
			continue
		}
		for dep := range man.Dependencies {
			add(databases[dep])
		}
	}
	for _, man := range set.Go {
		if !inScope(man.Dir) {
			continue
		}
		for dep := range man.Requirements {
			if mapped, ok := databases[dep]; ok {
				add(mapped)
				continue
			}
			add(databases[filepath.ToSlash(dep)])
		}
	}
	for _, man := range set.Rust {
		if !inScope(man.Dir) {
			continue
		}
		for dep := range man.Dependencies {
			add(databases[dep])
		}
	}
	return out
}

func Proven(c Candidate) bool {
	for _, ev := range c.Evidence {
		switch ev.Kind {
		case stack.KindDependency, stack.KindConfig, stack.KindEntrypoint:
			return true
		}
	}
	return false
}
