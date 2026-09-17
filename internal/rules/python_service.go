package rules

import (
	"path/filepath"

	"detection_engine/internal/index"
	"detection_engine/internal/manifests"
	"detection_engine/internal/stack"
)

func matchPythonService(idx *index.Index, set *manifests.Set) []Candidate {
	if len(set.Python) == 0 {
		return nil
	}
	var out []Candidate
	for _, man := range set.Python {
		if namedPythonFramework(man) {
			continue
		}
		entry, ok := pythonEntrypoint(idx, man.Dir)
		if !ok {
			continue
		}
		profile := Profile{
			Name:      "python-worker",
			Ecosystem: "python",
			Kind:      "framework",
			Runtime:   "python",
			Start:     "python -u " + filepath.Base(entry.Rel),
			Build:     "pip install -r requirements.txt",
		}
		if docker, ok := dockerHint(set); ok {
			if docker.Cmd != "" {
				profile.Start = docker.Cmd
			}
			if len(docker.Expose) > 0 {
				profile.Port = docker.Expose[0]
			}
		}
		if profile.Port == 0 {
			profile.Port = envPort(set)
		}
		c := Candidate{Profile: profile, Dir: man.Dir}
		c.add(stack.LayerManifest, stack.KindFile, man.Path, man.Kind, 25)
		c.add(stack.LayerRule, stack.KindEntrypoint, entry.Rel, "python entry "+entry.Rel, 25)
		if docker, ok := dockerHint(set); ok && (docker.Cmd != "" || docker.BuildCmd != "" || docker.From != "") {
			c.add(stack.LayerCatalog, stack.KindConfig, docker.Path, "container worker", 25)
		}
		out = append(out, c)
	}
	return out
}

func namedPythonFramework(man manifests.PythonManifest) bool {
	for _, profile := range profiles {
		if profile.Ecosystem != "python" {
			continue
		}
		for _, dep := range profile.Deps {
			if _, ok := man.Dependencies[dep]; ok {
				return true
			}
		}
	}
	return false
}

func pythonEntrypoint(idx *index.Index, root string) (index.File, bool) {
	for _, name := range []string{"main.py", "app.py", "worker.py"} {
		if file, ok := idx.HasInDir(root, name); ok {
			return file, true
		}
	}
	return index.File{}, false
}

