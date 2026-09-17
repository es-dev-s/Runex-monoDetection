package catalog

import (
	"strings"

	"detection_engine/internal/index"
	"detection_engine/internal/stack"
)

type Hit struct {
	Name     string
	Category string
	Path     string
}

func Scan(idx *index.Index) []Hit {
	var hits []Hit
	seen := map[string]struct{}{}
	add := func(name, category, path string) {
		key := name + "|" + path
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		hits = append(hits, Hit{Name: name, Category: category, Path: path})
	}

	for _, file := range idx.Files {
		name := strings.ToLower(file.Name)
		switch name {
		case "dockerfile", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
			add("docker", "infra", file.Rel)
		case "vercel.json", "vercel.ts":
			add("vercel", "infra", file.Rel)
		case "netlify.toml":
			add("netlify", "infra", file.Rel)
		case "fly.toml":
			add("fly.io", "infra", file.Rel)
		case "render.yaml", "render.yml":
			add("render", "infra", file.Rel)
		case "railway.toml", "railway.json":
			add("railway", "infra", file.Rel)
		case "firebase.json":
			add("firebase", "infra", file.Rel)
		case "turbo.json":
			add("turborepo", "build", file.Rel)
		case "nx.json":
			add("nx", "build", file.Rel)
		case "pnpm-workspace.yaml":
			add("pnpm-workspace", "monorepo", file.Rel)
		case "lerna.json":
			add("lerna", "monorepo", file.Rel)
		}
		if strings.HasSuffix(name, ".tf") {
			add("terraform", "infra", file.Rel)
		}
		if file.Rel == "prisma/schema.prisma" || strings.HasSuffix(file.Rel, "/prisma/schema.prisma") {
			add("prisma", "orm", file.Rel)
		}
		if strings.Contains(file.Rel, ".github/workflows/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
			add("github-actions", "infra", file.Rel)
		}
	}

	if _, ok := idx.HasInDir("", "next.config.ts", "next.config.js", "next.config.mjs", "next.config.cjs"); ok {
		add("nextjs", "config", "next.config.*")
	}
	return hits
}

func Infra(hits []Hit) []stack.Tech {
	seen := map[string]struct{}{}
	out := make([]stack.Tech, 0)
	for _, hit := range hits {
		if hit.Category != "infra" {
			continue
		}
		if _, ok := seen[hit.Name]; ok {
			continue
		}
		seen[hit.Name] = struct{}{}
		out = append(out, stack.Tech{Name: hit.Name})
	}
	return out
}
