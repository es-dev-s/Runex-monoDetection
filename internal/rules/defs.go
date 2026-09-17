package rules

type Profile struct {
	Name        string
	Ecosystem   string
	Kind        string // framework | library | build
	Runtime     string
	RuntimeFrom string
	Deps        []string
	GoModules   []string
	Configs     []string
	Scripts     []string
	Files       []string
	Port        int
	Build       string
	Start       string
	OutputDir   string
	Suppresses  []string
}

var profiles = []Profile{
	{
		Name: "nextjs", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"next"}, Configs: []string{"next.config.js", "next.config.ts", "next.config.mjs", "next.config.cjs"},
		Scripts: []string{"next build", "next start", "next dev"}, Port: 3000, Build: "next build", Start: "next start",
		OutputDir: ".next", Suppresses: []string{"react", "vite", "vue", "svelte"},
	},
	{
		Name: "remix", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"@remix-run/node", "@remix-run/react", "remix"}, Configs: []string{"remix.config.js", "remix.config.ts"},
		Scripts: []string{"remix build", "remix dev"}, Port: 3000, Build: "remix build", Start: "remix-serve build/index.js",
		Suppresses: []string{"react", "vite"},
	},
	{
		Name: "nuxt", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"nuxt"}, Configs: []string{"nuxt.config.js", "nuxt.config.ts"},
		Scripts: []string{"nuxt build", "nuxt dev"}, Port: 3000, Build: "nuxt build", Start: "node .output/server/index.mjs",
		OutputDir: ".output", Suppresses: []string{"vue", "vite"},
	},
	{
		Name: "sveltekit", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"@sveltejs/kit"}, Configs: []string{"svelte.config.js", "svelte.config.ts"},
		Scripts: []string{"svelte-kit"}, Port: 5173, Build: "vite build", Start: "node build",
		Suppresses: []string{"svelte", "vite", "react"},
	},
	{
		Name: "astro", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"astro"}, Configs: []string{"astro.config.mjs", "astro.config.ts", "astro.config.js"},
		Scripts: []string{"astro build", "astro dev"}, Port: 4321, Build: "astro build", Start: "astro preview",
		OutputDir: "dist",
	},
	{
		Name: "gatsby", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"gatsby"}, Configs: []string{"gatsby-config.js", "gatsby-config.ts"},
		Scripts: []string{"gatsby build", "gatsby develop"}, Port: 9000, Build: "gatsby build", Start: "gatsby serve",
		Suppresses: []string{"react"},
	},
	{
		Name: "nestjs", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"@nestjs/core"}, Configs: []string{"nest-cli.json"},
		Scripts: []string{"nest build", "nest start"}, Port: 3000, Build: "nest build", Start: "node dist/main",
		OutputDir: "dist", Suppresses: []string{"express"},
	},
	{
		Name: "express", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"express"}, Port: 3000, Start: "node index.js",
	},
	{
		Name: "fastify", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"fastify"}, Port: 3000, Start: "node index.js",
	},
	{
		Name: "hono", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"hono"}, Port: 3000, Start: "node index.js",
	},
	{
		Name: "koa", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"koa"}, Port: 3000, Start: "node index.js",
	},
	{
		Name: "angular", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"@angular/core"}, Configs: []string{"angular.json"},
		Scripts: []string{"ng build", "ng serve"}, Port: 4200, Build: "ng build", Start: "ng serve",
		OutputDir: "dist",
	},
	{
		Name: "vite", Ecosystem: "node", Kind: "build", Runtime: "nodejs",
		Deps: []string{"vite"}, Configs: []string{"vite.config.ts", "vite.config.js", "vite.config.mjs"},
		Scripts: []string{"vite build", "vite"}, Port: 5173, Build: "vite build", Start: "vite preview",
		OutputDir: "dist",
	},
	{
		Name: "react", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"react"}, Port: 5173, Build: "vite build", Start: "vite preview", OutputDir: "dist",
	},
	{
		Name: "vue", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"vue"}, Port: 5173, Build: "vite build", Start: "vite preview", OutputDir: "dist",
	},
	{
		Name: "svelte", Ecosystem: "node", Kind: "framework", Runtime: "nodejs",
		Deps: []string{"svelte"}, Port: 5173, Build: "vite build", Start: "vite preview", OutputDir: "dist",
	},
	{
		Name: "django", Ecosystem: "python", Kind: "framework", Runtime: "python",
		Deps: []string{"django"}, Files: []string{"manage.py"}, Port: 8000,
		Build: "", Start: "python manage.py runserver",
	},
	{
		Name: "fastapi", Ecosystem: "python", Kind: "framework", Runtime: "python",
		Deps: []string{"fastapi"}, Port: 8000, Start: "uvicorn main:app --host 0.0.0.0 --port 8000",
	},
	{
		Name: "flask", Ecosystem: "python", Kind: "framework", Runtime: "python",
		Deps: []string{"flask"}, Port: 5000, Start: "flask run --host 0.0.0.0",
	},
	{
		Name: "starlette", Ecosystem: "python", Kind: "framework", Runtime: "python",
		Deps: []string{"starlette"}, Port: 8000, Start: "uvicorn main:app --host 0.0.0.0 --port 8000",
	},
	{
		Name: "streamlit", Ecosystem: "python", Kind: "framework", Runtime: "python",
		Deps: []string{"streamlit"}, Port: 8501, Start: "streamlit run app.py",
	},
	{
		Name: "gin", Ecosystem: "go", Kind: "framework", Runtime: "go",
		GoModules: []string{"github.com/gin-gonic/gin"}, Port: 8080, Build: "go build -o app .", Start: "./app",
	},
	{
		Name: "echo", Ecosystem: "go", Kind: "framework", Runtime: "go",
		GoModules: []string{"github.com/labstack/echo", "github.com/labstack/echo/v4"}, Port: 8080, Build: "go build -o app .", Start: "./app",
	},
	{
		Name: "fiber", Ecosystem: "go", Kind: "framework", Runtime: "go",
		GoModules: []string{"github.com/gofiber/fiber", "github.com/gofiber/fiber/v2", "github.com/gofiber/fiber/v3"}, Port: 3000, Build: "go build -o app .", Start: "./app",
	},
	{
		Name: "chi", Ecosystem: "go", Kind: "framework", Runtime: "go",
		GoModules: []string{"github.com/go-chi/chi", "github.com/go-chi/chi/v5"}, Port: 8080, Build: "go build -o app .", Start: "./app",
	},
	{
		Name: "gorilla-mux", Ecosystem: "go", Kind: "framework", Runtime: "go",
		GoModules: []string{"github.com/gorilla/mux"}, Port: 8080, Build: "go build -o app .", Start: "./app",
	},
	{
		Name: "net/http", Ecosystem: "go", Kind: "framework", Runtime: "go",
		Port: 8080, Build: "go build -o app .", Start: "./app",
	},
	{
		Name: "axum", Ecosystem: "rust", Kind: "framework", Runtime: "rust",
		Deps: []string{"axum"}, Port: 3000, Build: "cargo build --release", Start: "./target/release/app",
	},
	{
		Name: "actix-web", Ecosystem: "rust", Kind: "framework", Runtime: "rust",
		Deps: []string{"actix-web"}, Port: 8080, Build: "cargo build --release", Start: "./target/release/app",
	},
	{
		Name: "rocket", Ecosystem: "rust", Kind: "framework", Runtime: "rust",
		Deps: []string{"rocket"}, Port: 8000, Build: "cargo build --release", Start: "./target/release/app",
	},
	{
		Name: "tauri", Ecosystem: "rust", Kind: "framework", Runtime: "rust",
		Deps: []string{"tauri"}, Configs: []string{"tauri.conf.json", "tauri.conf.json5"},
		Files: []string{"tauri.conf.json", "tauri.conf.json5"},
		Build: "cargo tauri build", Start: "cargo tauri dev",
	},
	{
		Name: "tauri", Ecosystem: "node", Kind: "library",
		Deps: []string{"@tauri-apps/api", "@tauri-apps/cli"},
	},
	{
		Name: "laravel", Ecosystem: "php", Kind: "framework", Runtime: "php",
		Deps: []string{"laravel/framework"}, Files: []string{"artisan"}, Port: 8000, Start: "php artisan serve",
	},
	{
		Name: "symfony", Ecosystem: "php", Kind: "framework", Runtime: "php",
		Deps: []string{"symfony/framework-bundle"}, Port: 8000, Start: "php -S 0.0.0.0:8000 -t public",
	},
	{
		Name: "rails", Ecosystem: "ruby", Kind: "framework", Runtime: "ruby",
		Deps: []string{"rails"}, Files: []string{"config.ru", "Rakefile"}, Port: 3000, Start: "bundle exec rails server -b 0.0.0.0",
	},
	{
		Name: "sinatra", Ecosystem: "ruby", Kind: "framework", Runtime: "ruby",
		Deps: []string{"sinatra"}, Port: 4567, Start: "bundle exec ruby app.rb",
	},
	{
		Name: "spring-boot", Ecosystem: "java", Kind: "framework", Runtime: "java",
		Deps: []string{"org.springframework.boot", "org.springframework.boot:spring-boot-starter-web", "spring-boot"},
		Files: []string{"mvnw", "gradlew"}, Port: 8080, Build: "mvn -q -DskipTests package", Start: "java -jar target/app.jar",
	},
}

var databases = map[string]string{
	"pg": "postgresql", "postgres": "postgresql", "postgresql": "postgresql",
	"psycopg": "postgresql", "psycopg2": "postgresql", "psycopg2-binary": "postgresql",
	"asyncpg": "postgresql", "pgx": "postgresql",
	"mongoose": "mongodb", "mongodb": "mongodb", "mongo": "mongodb",
	"redis": "redis", "ioredis": "redis", "go-redis": "redis",
	"mysql": "mysql", "mysql2": "mysql", "mariadb": "mysql",
	"sqlite": "sqlite", "sqlite3": "sqlite", "better-sqlite3": "sqlite",
	"prisma": "prisma",
	"sqlalchemy": "sqlalchemy", "diesel": "diesel", "sqlx": "sqlx",
	"github.com/jackc/pgx": "postgresql", "github.com/jackc/pgx/v5": "postgresql",
	"github.com/lib/pq": "postgresql", "github.com/go-sql-driver/mysql": "mysql",
	"github.com/redis/go-redis/v9": "redis",
}

