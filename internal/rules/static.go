package rules

import (
	"path/filepath"
	"strings"

	"detection_engine/internal/index"
	"detection_engine/internal/manifests"
	"detection_engine/internal/stack"
)

var skipStaticDirs = map[string]struct{}{
	"node_modules": {}, "dist": {}, "build": {}, "public": {}, "coverage": {},
	".next": {}, ".nuxt": {}, ".output": {}, "vendor": {}, "target": {},
}

func matchStatic(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	out = append(out, matchHTML(idx, set)...)
	out = append(out, matchNodeScript(idx, set)...)
	out = append(out, matchPlainJS(idx, set)...)
	return out
}

func matchHTML(idx *index.Index, set *manifests.Set) []Candidate {
	byDir := map[string][]index.File{}
	for _, file := range idx.FindSuffix(".html") {
		if skippedStaticDir(file.Dir) {
			continue
		}
		if _, ok := set.NodeInDir(file.Dir); ok {
			continue
		}
		byDir[file.Dir] = append(byDir[file.Dir], file)
	}
	var out []Candidate
	for dir, files := range byDir {
		entry := pickHTML(files)
		c := Candidate{
			Profile: Profile{
				Name:      "html",
				Ecosystem: "html",
				Kind:      "framework",
				Runtime:   "static",
				Port:      80,
				Start:     "npx serve " + displayDir(dir),
			},
			Dir: dir,
		}
		c.add(stack.LayerRule, stack.KindEntrypoint, entry.Rel, entry.Name, 50)
		if css, ok := firstSuffixInDir(idx, dir, ".css"); ok {
			c.add(stack.LayerRule, stack.KindFile, css.Rel, css.Name, 25)
		}
		if js, ok := firstSuffixInDir(idx, dir, ".js"); ok {
			c.add(stack.LayerRule, stack.KindFile, js.Rel, js.Name, 15)
		}
		out = append(out, c)
	}
	return out
}

func matchNodeScript(idx *index.Index, set *manifests.Set) []Candidate {
	var out []Candidate
	for _, man := range set.Node {
		if namedNodeFramework(man) || isWorkspaceRoot(man) {
			continue
		}
		entry := man.Main
		if entry == "" {
			entry = "index.js"
		}
		start := "node " + entry
		if cmd, ok := man.Scripts["start"]; ok && cmd != "" {
			start = cmd
		}
		c := Candidate{
			Profile: Profile{
				Name:      "nodejs",
				Ecosystem: "node",
				Kind:      "framework",
				Runtime:   "nodejs",
				Start:     start,
			},
			Dir: man.Dir,
		}
		c.add(stack.LayerManifest, stack.KindEntrypoint, man.Path, "package.json", 50)
		if file, ok := idx.HasInDir(man.Dir, filepath.Base(entry), "index.js", "server.js", "app.js"); ok {
			c.add(stack.LayerRule, stack.KindFile, file.Rel, file.Name, 25)
		}
		if _, ok := man.Scripts["start"]; ok {
			c.add(stack.LayerRule, stack.KindScript, man.Path, "scripts.start", 25)
		}
		out = append(out, c)
	}
	return out
}

func matchPlainJS(idx *index.Index, set *manifests.Set) []Candidate {
	byDir := map[string][]index.File{}
	for _, file := range idx.FindSuffix(".js") {
		if skippedStaticDir(file.Dir) || strings.HasSuffix(file.Name, ".min.js") {
			continue
		}
		if set.NodeOwnsDir(file.Dir) {
			continue
		}
		if hasHTMLInDir(idx, file.Dir) {
			continue
		}
		if nestedSourceDir(file.Dir) {
			continue
		}
		byDir[file.Dir] = append(byDir[file.Dir], file)
	}
	var out []Candidate
	for dir, files := range byDir {
		httpFile, isHTTP := jsLooksLikeHTTP(idx, files)
		if dir != "" && !isHTTP {
			continue
		}
		entry := files[0]
		name := "javascript"
		start := "node " + entry.Name
		port := 0
		if isHTTP {
			name = "nodejs"
			entry = httpFile
			start = "node " + httpFile.Name
			port = 3000
		}
		c := Candidate{
			Profile: Profile{
				Name:      name,
				Ecosystem: "node",
				Kind:      "framework",
				Runtime:   "nodejs",
				Start:     start,
				Port:      port,
			},
			Dir: dir,
		}
		c.add(stack.LayerRule, stack.KindEntrypoint, entry.Rel, entry.Name, 50)
		out = append(out, c)
	}
	return out
}

func namedNodeFramework(man manifests.NodeManifest) bool {
	for _, profile := range profiles {
		if profile.Ecosystem != "node" || profile.Kind != "framework" {
			continue
		}
		if profile.Name == "nodejs" || profile.Name == "javascript" || profile.Name == "html" {
			continue
		}
		for _, dep := range profile.Deps {
			if _, ok := man.Dep(dep); ok {
				return true
			}
		}
	}
	return false
}

func isWorkspaceRoot(man manifests.NodeManifest) bool {
	if len(man.Workspaces) > 0 {
		return true
	}
	if man.Main != "" {
		return false
	}
	if len(man.Scripts) == 0 {
		return man.Name != "" && len(man.Dependencies) == 0
	}
	delegating := 0
	for _, script := range man.Scripts {
		if strings.Contains(script, "--prefix") || strings.Contains(script, "concurrently") {
			delegating++
		}
	}
	return delegating > 0 && delegating == len(man.Scripts)
}

func skippedStaticDir(dir string) bool {
	for _, part := range strings.Split(dir, "/") {
		if _, skip := skipStaticDirs[part]; skip {
			return true
		}
	}
	return false
}

var nestedSourceNames = map[string]struct{}{
	"src": {}, "lib": {}, "libs": {}, "components": {}, "hooks": {},
	"utils": {}, "helpers": {}, "routes": {}, "middleware": {},
	"tests": {}, "test": {}, "__tests__": {}, "services": {},
	"validators": {}, "prompts": {}, "data": {}, "db": {},
	"config": {}, "pages": {}, "features": {}, "constants": {},
	"types": {}, "models": {}, "controllers": {}, "views": {},
	"stores": {}, "shared": {}, "internal": {}, "pkg": {},
}

func nestedSourceDir(dir string) bool {
	if dir == "" {
		return false
	}
	for _, part := range strings.Split(dir, "/") {
		if _, skip := nestedSourceNames[part]; skip {
			return true
		}
	}
	return false
}

func pickHTML(files []index.File) index.File {
	for _, file := range files {
		if strings.EqualFold(file.Name, "index.html") {
			return file
		}
	}
	return files[0]
}

func firstSuffixInDir(idx *index.Index, dir, ext string) (index.File, bool) {
	for _, file := range idx.FindSuffix(ext) {
		if file.Dir == dir {
			return file, true
		}
	}
	return index.File{}, false
}

func hasHTMLInDir(idx *index.Index, dir string) bool {
	_, ok := firstSuffixInDir(idx, dir, ".html")
	return ok
}

func jsLooksLikeHTTP(idx *index.Index, files []index.File) (index.File, bool) {
	needles := []string{"express", "createServer", "ListenAndServe", "http.Server", "listen("}
	for _, file := range files {
		data, err := index.ReadAbs(file.Abs, 32<<10)
		if err != nil {
			continue
		}
		text := string(data)
		for _, needle := range needles {
			if strings.Contains(text, needle) {
				return file, true
			}
		}
	}
	return index.File{}, false
}

func displayDir(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}
