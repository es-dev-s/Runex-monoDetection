package rules

import (
	"regexp"
	"strconv"

	"detection_engine/internal/index"
	"detection_engine/internal/manifests"
)

var (
	portAssign = regexp.MustCompile(`(?i)\bPORT\s*[=:]\s*(\d{2,5})`)
	portOr     = regexp.MustCompile(`(?i)PORT\)?\s*\|\|\s*(\d{2,5})`)
	listenPort = regexp.MustCompile(`(?i)listen\(\s*(\d{2,5})\s*`)
)

func NodeListenPort(idx *index.Index, set *manifests.Set, dir string) int {
	if port := envPortIn(set, dir); port > 0 {
		return port
	}
	if node, ok := set.NodeInDir(dir); ok {
		for _, script := range node.Scripts {
			if match := portAssign.FindStringSubmatch(script); len(match) == 2 {
				if port, err := strconv.Atoi(match[1]); err == nil {
					return port
				}
			}
		}
	}
	for _, name := range []string{"index.js", "server.js", "app.js", "main.js"} {
		file, ok := idx.HasInDir(dir, name)
		if !ok {
			continue
		}
		data, err := index.ReadAbs(file.Abs, 32<<10)
		if err != nil {
			continue
		}
		text := string(data)
		for _, re := range []*regexp.Regexp{portOr, portAssign, listenPort} {
			if match := re.FindStringSubmatch(text); len(match) == 2 {
				if port, err := strconv.Atoi(match[1]); err == nil && port > 0 {
					return port
				}
			}
		}
	}
	return 0
}

func envPortIn(set *manifests.Set, dir string) int {
	for _, env := range set.Env {
		if env.Dir != dir && env.Dir != "" && dir != "" {
			continue
		}
		if port := portFromMap(env.Vars); port > 0 {
			return port
		}
	}
	return 0
}

