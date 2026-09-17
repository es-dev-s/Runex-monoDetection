package rules

import (
	"path"
	"strconv"
	"strings"

	"detection_engine/internal/index"
	"detection_engine/internal/manifests"
	"detection_engine/internal/stack"
)

var httpNeedles = []string{
	`"net/http"`,
	"ListenAndServe",
	"http.Server",
	"http.NewServeMux",
	"http.HandleFunc",
	"http.Handle(",
}

func matchGoService(idx *index.Index, set *manifests.Set) []Candidate {
	if len(set.Go) == 0 {
		return nil
	}

	var out []Candidate
	for _, man := range set.Go {
		if namedGoFramework(set, man.Dir) {
			continue
		}
		entry, ok := goEntrypoint(idx, man.Dir)
		if !ok {
			continue
		}

		profile := netHTTPProfile(set, entry)
		c := Candidate{Profile: profile, Dir: man.Dir}
		c.add(stack.LayerManifest, stack.KindFile, man.Path, "go.mod "+man.Module, 25)
		c.add(stack.LayerRule, stack.KindEntrypoint, entry.Rel, "main package "+entry.Rel, 25)

		if file, ok := scanGoHTTP(idx); ok {
			c.add(stack.LayerRule, stack.KindConfig, file.Rel, "net/http server", 25)
		} else if strings.HasPrefix(entry.Dir, "cmd/") || strings.Contains(entry.Dir, "/server") {
			c.add(stack.LayerRule, stack.KindConfig, entry.Rel, "cmd server layout", 25)
		}

		if docker, ok := dockerHint(set); ok {
			if docker.BuildCmd != "" {
				c.add(stack.LayerCatalog, stack.KindScript, docker.Path, docker.BuildCmd, 25)
			} else if len(docker.Expose) > 0 {
				c.add(stack.LayerCatalog, stack.KindScript, docker.Path, "EXPOSE "+strconv.Itoa(docker.Expose[0]), 15)
			}
		}
		if mk, ok := makeHint(set); ok && mk.Build != "" {
			c.add(stack.LayerCatalog, stack.KindScript, mk.Path, mk.Build, 15)
		}
		if c.Score > 0 {
			out = append(out, c)
		}
	}
	return out
}

func namedGoFramework(set *manifests.Set, dir string) bool {
	for _, man := range set.Go {
		if man.Dir != dir {
			continue
		}
		for _, profile := range profiles {
			if profile.Ecosystem != "go" || profile.Name == "net/http" {
				continue
			}
			for _, mod := range profile.GoModules {
				if _, ok := man.Requirements[mod]; ok {
					return true
				}
			}
		}
	}
	return false
}

func netHTTPProfile(set *manifests.Set, entry index.File) Profile {
	p := Profile{
		Name:      "net/http",
		Ecosystem: "go",
		Kind:      "framework",
		Runtime:   "go",
		Port:      8080,
		Build:     "go build -o app " + goPackage(entry),
		Start:     "./app",
	}
	if mk, ok := makeHint(set); ok {
		if mk.Build != "" {
			p.Build = mk.Build
		}
		if mk.Binary != "" {
			p.Start = startBinary(mk.Binary)
		} else if mk.Run != "" {
			p.Start = mk.Run
		}
		if mk.Package != "" && !strings.Contains(p.Build, mk.Package) {
			p.Build = mk.Build
		}
	}
	if docker, ok := dockerHint(set); ok {
		if docker.BuildCmd != "" {
			p.Build = docker.BuildCmd
		}
		if docker.Entrypoint != "" {
			p.Start = startBinary(docker.Entrypoint)
		} else if docker.Cmd != "" {
			p.Start = startBinary(docker.Cmd)
		} else if docker.Binary != "" {
			p.Start = startBinary(docker.Binary)
		}
		if port := dockerPort(docker, set); port > 0 {
			p.Port = port
		}
	} else if port := envPort(set); port > 0 {
		p.Port = port
	}
	return p
}

func goEntrypoint(idx *index.Index, root string) (index.File, bool) {
	var mains []index.File
	for _, file := range idx.FindName("main.go") {
		rel := file.Rel
		if strings.Contains(rel, "/testdata/") || strings.Contains(rel, "/vendor/") ||
			strings.Contains(rel, "/examples/") || strings.Contains(rel, "/tools/") {
			continue
		}
		if root != "" && file.Dir != root && !strings.HasPrefix(file.Dir, root+"/") {
			continue
		}
		mains = append(mains, file)
	}
	if len(mains) == 0 {
		return index.File{}, false
	}
	best := mains[0]
	bestScore := entryScore(best)
	for _, file := range mains[1:] {
		if s := entryScore(file); s > bestScore {
			best, bestScore = file, s
		}
	}
	return best, true
}

func entryScore(file index.File) int {
	dir := file.Dir
	switch {
	case dir == "cmd/server" || strings.HasSuffix(dir, "/cmd/server"):
		return 100
	case dir == "cmd/api" || dir == "cmd/app" || dir == "cmd/http" || dir == "cmd/signaling":
		return 90
	case strings.HasPrefix(dir, "cmd/"):
		return 70
	case dir == "":
		return 40
	default:
		return 10
	}
}

func goPackage(file index.File) string {
	if file.Dir == "" {
		return "."
	}
	return "./" + file.Dir
}

func scanGoHTTP(idx *index.Index) (index.File, bool) {
	for _, file := range idx.Files {
		if !strings.HasSuffix(file.Name, ".go") || strings.HasSuffix(file.Name, "_test.go") {
			continue
		}
		if strings.Contains(file.Rel, "/vendor/") {
			continue
		}
		data, err := index.ReadAbs(file.Abs, 64<<10)
		if err != nil {
			continue
		}
		text := string(data)
		for _, needle := range httpNeedles {
			if strings.Contains(text, needle) {
				return file, true
			}
		}
	}
	return index.File{}, false
}

func dockerHint(set *manifests.Set) (manifests.DockerManifest, bool) {
	for _, docker := range set.Docker {
		if docker.BuildCmd != "" || docker.Entrypoint != "" || len(docker.Expose) > 0 {
			return docker, true
		}
	}
	if len(set.Docker) > 0 {
		return set.Docker[0], true
	}
	return manifests.DockerManifest{}, false
}

func makeHint(set *manifests.Set) (manifests.MakeManifest, bool) {
	if len(set.Make) == 0 {
		return manifests.MakeManifest{}, false
	}
	return set.Make[0], true
}

func dockerPort(docker manifests.DockerManifest, set *manifests.Set) int {
	if len(docker.Expose) > 0 {
		return docker.Expose[0]
	}
	if port := portFromMap(docker.Env); port > 0 {
		return port
	}
	return envPort(set)
}

func envPort(set *manifests.Set) int {
	for _, env := range set.Env {
		if port := portFromMap(env.Vars); port > 0 {
			return port
		}
	}
	return 0
}

func portFromMap(vars map[string]string) int {
	if vars == nil {
		return 0
	}
	for _, key := range []string{"ADDR", "PORT", "HTTP_PORT", "APP_PORT"} {
		if port := parseListenPort(vars[key]); port > 0 {
			return port
		}
	}
	return 0
}

func parseListenPort(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		raw = raw[i+1:]
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 65535 {
		return 0
	}
	return n
}

func startBinary(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "./app"
	}
	base := path.Base(raw)
	if strings.HasPrefix(raw, "/") {
		return raw
	}
	if strings.HasPrefix(raw, "./") {
		return raw
	}
	return "./" + base
}
