package stack

const (
	LayerLinguist = "linguist"
	LayerManifest = "manifest"
	LayerCatalog  = "catalog"
	LayerRule     = "rule"
)

const (
	KindLanguage   = "language"
	KindDependency = "dependency"
	KindLockfile   = "lockfile"
	KindConfig     = "config"
	KindScript     = "script"
	KindFile       = "file"
	KindRuntime    = "runtime"
	KindEntrypoint = "entrypoint"
)

type SourceType string

const (
	SourceGitHub SourceType = "github"
	SourceZip    SourceType = "zip"
	SourceDir    SourceType = "dir"
)

type Source struct {
	Type   SourceType `json:"type"`
	URL    string     `json:"url,omitempty"`
	Ref    string     `json:"ref,omitempty"`
	Commit string     `json:"commit,omitempty"`
	Name   string     `json:"name,omitempty"`
}

type Language struct {
	Name       string  `json:"name"`
	Bytes      int64   `json:"bytes"`
	Percentage float64 `json:"percentage"`
}

type Tech struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type Command struct {
	Command   string `json:"command,omitempty"`
	OutputDir string `json:"outputDir,omitempty"`
}

type Evidence struct {
	Layer  string `json:"layer"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
	Weight int    `json:"weight"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type App struct {
	Path           string     `json:"path"`
	Language       string     `json:"language,omitempty"`
	Runtime        *Tech      `json:"runtime,omitempty"`
	Framework      *Tech      `json:"framework,omitempty"`
	Libraries      []Tech     `json:"libraries,omitempty"`
	PackageManager *Tech      `json:"packageManager,omitempty"`
	BuildTool      *Tech      `json:"buildTool,omitempty"`
	Build          Command    `json:"build"`
	Start          Command    `json:"start"`
	Port           int        `json:"port,omitempty"`
	Databases      []Tech     `json:"databases,omitempty"`
	Confidence     int        `json:"confidence"`
	Evidence       []Evidence `json:"evidence"`
}

type Result struct {
	RequestID  string    `json:"requestId"`
	DurationMS int64     `json:"durationMs"`
	Source     Source    `json:"source"`
	Languages  []Language `json:"languages"`
	Apps       []App     `json:"apps"`
	Infra      []Tech    `json:"infra"`
	Warnings   []Warning `json:"warnings"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorBody struct {
	Error APIError `json:"error"`
}
