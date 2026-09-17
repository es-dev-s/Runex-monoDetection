package index

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"detection_engine/internal/config"
)

var skipDirNames = map[string]struct{}{
	".git": {}, "node_modules": {}, "vendor": {}, "dist": {}, "build": {},
	".next": {}, ".nuxt": {}, ".output": {}, ".turbo": {}, ".cache": {},
	"target": {}, "__pycache__": {}, ".venv": {}, "venv": {}, ".tox": {},
	"coverage": {}, ".idea": {}, ".vscode": {},
}

type File struct {
	Rel      string
	Abs      string
	Name     string
	Dir      string
	Size     int64
	IsDir    bool
}

type Index struct {
	Root  string
	Files []File
	byRel map[string]File
	byName map[string][]File
}

func Build(ctx context.Context, root string, cfg config.Config) (*Index, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	idx := &Index{
		Root:   abs,
		byRel:  make(map[string]File),
		byName: make(map[string][]File),
	}

	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if _, skip := skipDirNames[d.Name()]; skip && path != abs {
				return filepath.SkipDir
			}
			return nil
		}
		if len(idx.Files) >= cfg.MaxWalkFiles {
			return fs.SkipAll
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		file := File{
			Rel:  rel,
			Abs:  path,
			Name: d.Name(),
			Dir:  filepath.ToSlash(filepath.Dir(rel)),
			Size: info.Size(),
		}
		if file.Dir == "." {
			file.Dir = ""
		}
		idx.Files = append(idx.Files, file)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(idx.Files, func(i, j int) bool { return idx.Files[i].Rel < idx.Files[j].Rel })
	for _, file := range idx.Files {
		idx.byRel[file.Rel] = file
		idx.byName[strings.ToLower(file.Name)] = append(idx.byName[strings.ToLower(file.Name)], file)
	}
	return idx, nil
}

func (idx *Index) FindName(name string) []File {
	return idx.byName[strings.ToLower(name)]
}

func (idx *Index) FindNames(names ...string) []File {
	var out []File
	seen := make(map[string]struct{})
	for _, name := range names {
		for _, file := range idx.FindName(name) {
			if _, ok := seen[file.Rel]; ok {
				continue
			}
			seen[file.Rel] = struct{}{}
			out = append(out, file)
		}
	}
	return out
}

func (idx *Index) HasName(name string) bool {
	return len(idx.FindName(name)) > 0
}

func (idx *Index) FindSuffix(ext string) []File {
	ext = strings.ToLower(ext)
	var out []File
	for _, file := range idx.Files {
		if strings.ToLower(filepath.Ext(file.Name)) == ext {
			out = append(out, file)
		}
	}
	return out
}

func (idx *Index) FilesInDir(dir string) []File {
	dir = filepath.ToSlash(dir)
	if dir == "." {
		dir = ""
	}
	var out []File
	for _, file := range idx.Files {
		if file.Dir == dir {
			out = append(out, file)
		}
	}
	return out
}

func (idx *Index) HasInDir(dir string, names ...string) (File, bool) {
	dir = filepath.ToSlash(dir)
	if dir == "." {
		dir = ""
	}
	want := make(map[string]struct{}, len(names))
	for _, name := range names {
		want[strings.ToLower(name)] = struct{}{}
	}
	for _, file := range idx.Files {
		if file.Dir != dir {
			continue
		}
		if _, ok := want[strings.ToLower(file.Name)]; ok {
			return file, true
		}
	}
	return File{}, false
}

func (idx *Index) Read(rel string, limit int64) ([]byte, error) {
	file, ok := idx.byRel[filepath.ToSlash(rel)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return ReadAbs(file.Abs, limit)
}

func ReadAbs(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, err
	}
	return buf[:n], nil
}

func RelDir(rel string) string {
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." {
		return ""
	}
	return dir
}
