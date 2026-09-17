package score

import (
	"sort"

	"detection_engine/internal/index"
	"detection_engine/internal/linguist"
	"detection_engine/internal/manifests"
	"detection_engine/internal/rules"
	"detection_engine/internal/stack"
)

func BuildApps(idx *index.Index, set *manifests.Set, langs []stack.Language, candidates []rules.Candidate) []stack.App {
	byDir := map[string][]rules.Candidate{}
	for _, c := range candidates {
		byDir[c.Dir] = append(byDir[c.Dir], c)
	}

	dirs := appDirs(set, byDir)
	apps := make([]stack.App, 0, len(dirs))
	for _, dir := range dirs {
		apps = append(apps, buildApp(idx, set, langs, byDir[dir], dir))
	}
	if len(apps) == 0 {
		apps = append(apps, languageOnlyApp(langs))
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Confidence == apps[j].Confidence {
			return apps[i].Path < apps[j].Path
		}
		return apps[i].Confidence > apps[j].Confidence
	})
	return apps
}

func appDirs(set *manifests.Set, byDir map[string][]rules.Candidate) []string {
	seen := map[string]struct{}{}
	var dirs []string
	add := func(dir string) {
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}

	rootHasWorkspaces := false
	for _, node := range set.Node {
		if node.Dir == "" && len(node.Workspaces) > 0 {
			rootHasWorkspaces = true
		}
	}
	if rootHasWorkspaces {
		for dir, cs := range byDir {
			if dir == "" {
				continue
			}
			if hasFramework(cs) {
				add(dir)
			}
		}
		if len(dirs) > 0 {
			return dirs
		}
	}
	if len(byDir) > 0 {
		for dir := range byDir {
			add(dir)
		}
		return dirs
	}
	if len(set.Node) > 0 || len(set.Python) > 0 || len(set.Go) > 0 || len(set.Rust) > 0 {
		add("")
	}
	return dirs
}

func hasFramework(cs []rules.Candidate) bool {
	for _, c := range cs {
		if c.Profile.Kind == "framework" {
			return true
		}
	}
	return false
}

func buildApp(idx *index.Index, set *manifests.Set, langs []stack.Language, cs []rules.Candidate, dir string) stack.App {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Score == cs[j].Score {
			return cs[i].Profile.Name < cs[j].Profile.Name
		}
		return cs[i].Score > cs[j].Score
	})

	suppressed := map[string]struct{}{}
	var framework *rules.Candidate
	var buildTool *rules.Candidate
	var libraries []stack.Tech
	for i := range cs {
		c := cs[i]
		if c.Profile.Kind == "framework" && framework == nil && rules.Proven(c) {
			framework = &cs[i]
			for _, name := range c.Profile.Suppresses {
				suppressed[name] = struct{}{}
			}
		}
	}
	for i := range cs {
		c := cs[i]
		if _, skip := suppressed[c.Profile.Name]; skip {
			continue
		}
		if c.Profile.Kind == "build" && buildTool == nil {
			buildTool = &cs[i]
		}
		if c.Profile.Kind == "library" {
			libraries = append(libraries, stack.Tech{Name: c.Profile.Name, Version: c.Version})
		}
	}

	app := stack.App{
		Path:      displayPath(dir),
		Language:  languageFor(langs, framework),
		Libraries: libraries,
		Databases: rules.DetectDatabases(set, dir),
		Evidence:  nil,
	}

	if framework != nil {
		app.Framework = &stack.Tech{Name: framework.Profile.Name, Version: framework.Version}
		app.Runtime = runtimeOf(set, dir, framework.Profile, langs)
		app.Port = framework.Profile.Port
		app.Build = commandFromScripts(set, dir, framework.Profile.Build, "build")
		if app.Build.Command == "" {
			app.Build = stack.Command{Command: framework.Profile.Build, OutputDir: framework.Profile.OutputDir}
		} else {
			app.Build.OutputDir = framework.Profile.OutputDir
		}
		app.Start = commandFromScripts(set, dir, framework.Profile.Start, "start")
		if app.Start.Command == "" {
			app.Start = stack.Command{Command: framework.Profile.Start}
		}
		app.Evidence = append(app.Evidence, framework.Evidence...)
		app.Confidence = confidence(framework)
	} else {
		app.Runtime = inferredRuntime(set, dir, langs)
		if name := stackNameFrom(set, langs, dir); name != "" {
			app.Framework = &stack.Tech{Name: name}
			app.Language = displayLanguage(name, langs)
			app.Confidence = 60
		} else {
			app.Confidence = 40
		}
		if app.Runtime != nil {
			app.Evidence = append(app.Evidence, stack.Evidence{
				Layer: stack.LayerLinguist, Kind: stack.KindRuntime, Path: app.Path,
				Detail: app.Runtime.Name, Weight: 20,
			})
		}
	}

	if buildTool != nil {
		app.BuildTool = &stack.Tech{Name: buildTool.Profile.Name, Version: buildTool.Version}
		app.Evidence = append(app.Evidence, buildTool.Evidence...)
		if app.Framework == nil {
			app.Port = buildTool.Profile.Port
			app.Build = stack.Command{Command: buildTool.Profile.Build, OutputDir: buildTool.Profile.OutputDir}
			app.Start = stack.Command{Command: buildTool.Profile.Start}
			app.Confidence = max(app.Confidence, confidence(buildTool)-10)
		}
	}

	if pm, path := manifests.PackageManager(idx, set, dir); pm != "" {
		app.PackageManager = &stack.Tech{Name: pm}
		app.Evidence = append(app.Evidence, stack.Evidence{
			Layer: stack.LayerManifest, Kind: stack.KindLockfile, Path: path,
			Detail: pm, Weight: 10,
		})
	}

	if app.Runtime != nil && app.Runtime.Name == "nodejs" {
		if port := rules.NodeListenPort(idx, set, dir); port > 0 {
			app.Port = port
		}
	}

	if node, ok := set.NodeInDir(dir); ok && node.Engines["node"] != "" && app.Runtime != nil && app.Runtime.Name == "nodejs" {
		app.Runtime.Version = manifests.NormalizeVersion(node.Engines["node"])
		app.Evidence = append(app.Evidence, stack.Evidence{
			Layer: stack.LayerManifest, Kind: stack.KindRuntime, Path: node.Path,
			Detail: "engines.node=" + node.Engines["node"], Weight: 10,
		})
	}
	if goMan := goInDir(set, dir); goMan != nil && app.Runtime != nil && app.Runtime.Name == "go" {
		app.Runtime.Version = goMan.GoVersion
		if goMan.GoVersion != "" {
			app.Evidence = append(app.Evidence, stack.Evidence{
				Layer: stack.LayerManifest, Kind: stack.KindRuntime, Path: goMan.Path,
				Detail: "go " + goMan.GoVersion, Weight: 10,
			})
		}
	}
	if py := pythonInDir(set, dir); py != nil && app.Runtime != nil && app.Runtime.Name == "python" && py.Requires != "" {
		app.Runtime.Version = manifests.NormalizeVersion(py.Requires)
		app.Evidence = append(app.Evidence, stack.Evidence{
			Layer: stack.LayerManifest, Kind: stack.KindRuntime, Path: py.Path,
			Detail: "requires-python=" + py.Requires, Weight: 10,
		})
	}

	if app.Confidence >= 100 {
		app.Confidence = 100
	}
	if app.Evidence == nil {
		app.Evidence = []stack.Evidence{}
	}
	if app.Libraries == nil {
		app.Libraries = []stack.Tech{}
	}
	if app.Databases == nil {
		app.Databases = []stack.Tech{}
	}
	return app
}

func confidence(c *rules.Candidate) int {
	kinds := map[string]struct{}{}
	sum := 0
	for _, ev := range c.Evidence {
		if _, ok := kinds[ev.Kind]; !ok {
			sum += ev.Weight
			kinds[ev.Kind] = struct{}{}
		}
	}
	_, hasDep := kinds[stack.KindDependency]
	_, hasEntry := kinds[stack.KindEntrypoint]
	if hasDep || hasEntry {
		if len(kinds) >= 3 {
			return 100
		}
		if len(kinds) == 2 {
			return 90
		}
		return 75
	}
	if sum > 100 {
		return 100
	}
	return sum
}

func commandFromScripts(set *manifests.Set, dir, fallback, scriptName string) stack.Command {
	if node, ok := set.NodeInDir(dir); ok {
		if cmd, ok := node.Scripts[scriptName]; ok && cmd != "" {
			return stack.Command{Command: cmd}
		}
	}
	return stack.Command{Command: fallback}
}

func runtimeOf(set *manifests.Set, dir string, profile rules.Profile, langs []stack.Language) *stack.Tech {
	if profile.Runtime != "" {
		return &stack.Tech{Name: profile.Runtime}
	}
	return inferredRuntime(set, dir, langs)
}

func inferredRuntime(set *manifests.Set, dir string, langs []stack.Language) *stack.Tech {
	if _, ok := set.NodeInDir(dir); ok {
		return &stack.Tech{Name: "nodejs"}
	}
	if pythonInDir(set, dir) != nil {
		return &stack.Tech{Name: "python"}
	}
	if goInDir(set, dir) != nil {
		return &stack.Tech{Name: "go"}
	}
	if rustInDir(set, dir) != nil {
		return &stack.Tech{Name: "rust"}
	}
	if name := rulesRuntimeFromLang(langs); name != "" {
		return &stack.Tech{Name: name}
	}
	for _, lang := range langs {
		if lang.Name == "HTML" {
			return &stack.Tech{Name: "static"}
		}
	}
	return nil
}

func rulesRuntimeFromLang(langs []stack.Language) string {
	if len(langs) == 0 {
		return ""
	}
	if runtime, ok := rulesRuntime(langs[0].Name); ok {
		return runtime
	}
	return ""
}

func rulesRuntime(lang string) (string, bool) {
	runtimes := map[string]string{
		"TypeScript": "nodejs", "JavaScript": "nodejs",
		"Python": "python", "Go": "go", "Rust": "rust",
		"PHP": "php", "Ruby": "ruby", "Java": "java",
		"Kotlin": "java", "C#": "dotnet",
	}
	v, ok := runtimes[lang]
	return v, ok
}

func languageFor(langs []stack.Language, framework *rules.Candidate) string {
	if framework != nil {
		switch framework.Profile.Ecosystem {
		case "node":
			if hasTS(langs) {
				return "TypeScript"
			}
			return "JavaScript"
		case "python":
			return "Python"
		case "go":
			return "Go"
		case "rust":
			return "Rust"
		case "php":
			return "PHP"
		case "ruby":
			return "Ruby"
		case "java":
			return "Java"
		case "html":
			return "HTML"
		}
	}
	return linguist.Primary(langs)
}

func displayLanguage(stackName string, langs []stack.Language) string {
	switch stackName {
	case "html":
		return "HTML"
	case "javascript":
		return "JavaScript"
	case "nodejs":
		if hasTS(langs) {
			return "TypeScript"
		}
		return "JavaScript"
	case "python", "python-worker":
		return "Python"
	case "go", "net/http":
		return "Go"
	}
	return linguist.Primary(langs)
}

func stackNameFrom(set *manifests.Set, langs []stack.Language, dir string) string {
	if _, ok := set.NodeInDir(dir); ok {
		return "nodejs"
	}
	if pythonInDir(set, dir) != nil {
		return "python"
	}
	if goInDir(set, dir) != nil {
		return "go"
	}
	if rustInDir(set, dir) != nil {
		return "rust"
	}
	for _, lang := range langs {
		switch lang.Name {
		case "HTML":
			return "html"
		case "JavaScript", "TypeScript":
			return "javascript"
		case "Python":
			return "python"
		case "Go":
			return "go"
		case "PHP":
			return "php"
		case "Ruby":
			return "ruby"
		case "Java":
			return "java"
		case "Rust":
			return "rust"
		}
	}
	return ""
}

func hasTS(langs []stack.Language) bool {
	for _, lang := range langs {
		if lang.Name == "TypeScript" {
			return true
		}
	}
	return false
}

func languageOnlyApp(langs []stack.Language) stack.App {
	app := stack.App{Path: ".", Language: linguist.Primary(langs), Confidence: 25}
	if runtime := inferredRuntime(&manifests.Set{}, "", langs); runtime != nil {
		app.Runtime = runtime
	}
	if name := stackNameFrom(&manifests.Set{}, langs, ""); name != "" {
		app.Framework = &stack.Tech{Name: name}
		app.Confidence = 60
	}
	if app.Language != "" {
		app.Evidence = []stack.Evidence{{
			Layer: stack.LayerLinguist, Kind: stack.KindLanguage, Path: ".",
			Detail: app.Language, Weight: 25,
		}}
	}
	return app
}

func displayPath(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}

func goInDir(set *manifests.Set, dir string) *manifests.GoManifest {
	for i := range set.Go {
		if set.Go[i].Dir == dir {
			return &set.Go[i]
		}
	}
	return nil
}

func pythonInDir(set *manifests.Set, dir string) *manifests.PythonManifest {
	for i := range set.Python {
		if set.Python[i].Dir == dir {
			return &set.Python[i]
		}
	}
	return nil
}

func rustInDir(set *manifests.Set, dir string) *manifests.RustManifest {
	for i := range set.Rust {
		if set.Rust[i].Dir == dir {
			return &set.Rust[i]
		}
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Warnings(apps []stack.App) []stack.Warning {
	out := make([]stack.Warning, 0)
	if len(apps) == 0 {
		out = append(out, stack.Warning{Code: "empty", Message: "No application stack could be determined"})
		return out
	}
	for _, app := range apps {
		if app.Framework == nil && app.Language == "" {
			out = append(out, stack.Warning{
				Code:    "empty",
				Message: "No source files were found in " + app.Path,
			})
		}
		if app.Confidence < 50 {
			out = append(out, stack.Warning{
				Code:    "low_confidence",
				Message: "Confidence for " + app.Path + " is below production threshold",
			})
		}
	}
	return out
}
