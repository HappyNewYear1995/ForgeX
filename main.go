package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"jenkinsAgent/config"
	"jenkinsAgent/internal/handler"
	"jenkinsAgent/internal/middleware"
	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/service"
	"jenkinsAgent/internal/store"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

func parseTemplates(funcMap template.FuncMap) map[string]*template.Template {
	tmpl := make(map[string]*template.Template)
	// Pages that use layout
	pages := []string{
		"dashboard", "product_list", "product_form", "product_detail",
		"release_list", "release_detail", "build_list", "config", "docs",
	}
	for _, p := range pages {
		t := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html", "templates/"+p+".html"))
		tmpl[p] = t
	}
	// Login is standalone (no layout)
	tmpl["login"] = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/login.html"))
	return tmpl
}

func main() {
	// Load config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Init database
	if err := store.Init(cfg.Database.Path); err != nil {
		log.Fatalf("failed to init database: %v", err)
	}

	// Init admin user
	if err := handler.InitAdmin(); err != nil {
		log.Fatalf("failed to init admin: %v", err)
	}

	// Parse templates - each page gets its own layout+content pair
	buildTime := fmt.Sprintf("%d", time.Now().Unix())
	funcMap := template.FuncMap{
		"printf":    fmt.Sprintf,
		"cacheBust": func() string { return buildTime },
	}
	tmpl := parseTemplates(funcMap)

	// Init services
	productService := service.NewProductService()
	releaseService := service.NewReleaseService()
	configService := service.NewConfigService()
	jenkinsService := service.NewJenkinsService(&cfg.Jenkins)
	jenkinsService.StartPolling(releaseService)
	buildService := service.NewBuildService(jenkinsService, releaseService)
	playwrightService := service.NewPlaywrightService()

	// Auto-run scripts after build completion
	testEnvStore := store.NewTestEnvStore()
	buildStore := store.NewBuildStore()
	jenkinsService.SetOnBuildComplete(func(b *model.Build) {
		if b.RunScriptsAfterBuild {
			playwrightService.RunScriptsForBuild(b, buildStore, testEnvStore)
		}
	})

	// Init handlers
	authHandler := handler.NewAuthHandler(tmpl)
	productHandler := handler.NewProductHandler(tmpl, productService, buildService, releaseService, configService)
	buildHandler := handler.NewBuildHandler(tmpl, buildService, releaseService, productService)
	releaseHandler := handler.NewReleaseHandler(tmpl, releaseService, productService)
	configHandler := handler.NewConfigHandler(tmpl, configService, jenkinsService, productService)
	testEnvHandler := handler.NewTestEnvHandler(configService, playwrightService)
	callbackHandler := handler.NewCallbackHandler()
	docsHandler := handler.NewDocsHandler(tmpl)

	// Setup static file server
	staticSub, _ := fs.Sub(staticFS, "static")
	staticServer := http.FileServer(http.FS(staticSub))

	// Setup router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", staticServer))

	// Public routes
	r.Get("/login", authHandler.LoginPage)
	r.Post("/login", authHandler.Login)
	r.Get("/logout", authHandler.Logout)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/products", http.StatusSeeOther)
		})

		// Products
		r.Route("/products", func(r chi.Router) {
			r.Get("/", productHandler.List)
			r.Get("/new", productHandler.CreateForm)
			r.Post("/new", productHandler.Create)
			r.Get("/{id}", productHandler.Detail)
			r.Get("/{id}/edit", productHandler.EditForm)
			r.Post("/{id}/edit", productHandler.Update)
			r.Post("/{id}/delete", productHandler.Delete)
			r.Post("/{id}/components", productHandler.AddComponent)
			r.Post("/{id}/components/{componentId}/delete", productHandler.DeleteComponent)
			r.Post("/{id}/build", buildHandler.Trigger)
			r.Get("/{id}/builds", buildHandler.List)
			r.Get("/{id}/releases/json", releaseHandler.ReleasesJSON)
			r.Post("/{id}/releases", releaseHandler.Create)
		})

		// Releases
		r.Route("/releases", func(r chi.Router) {
			r.Get("/", releaseHandler.List)
			r.Get("/{id}", releaseHandler.Detail)
		})

		// Builds
		r.Get("/builds/{buildId}/log", buildHandler.Log)
		r.Get("/builds/{buildId}/artifacts", buildHandler.Artifacts)
		r.Get("/builds/artifacts/{artifactId}/download", buildHandler.DownloadArtifact)

		// Docs
		r.Get("/docs", docsHandler.Page)
		r.Post("/docs/faq/new", docsHandler.CreateFAQ)
		r.Post("/docs/faq/{id}/delete", docsHandler.DeleteFAQ)
		r.Get("/docs/faq/json", docsHandler.ListFAQsJSON)

		// Config
		r.Get("/config", configHandler.Page)
		r.Get("/config/tree/json", configHandler.TreeJSON)
		r.Post("/config/new", configHandler.Create)
		r.Post("/config/{id}/edit", configHandler.Update)
		r.Post("/config/{id}/delete", configHandler.Delete)
		r.Post("/config/sys/{category}", configHandler.SaveSysConfig)
		r.Post("/config/sys/{category}/test", configHandler.TestAPI)
		r.Get("/api/gitea/repos", configHandler.GiteaRepos)
		r.Get("/api/gitea/branches", configHandler.GiteaBranches)

		// Test Environments
		r.Post("/config/testenv/new", configHandler.CreateTestEnv)
		r.Post("/config/testenv/{id}/edit", configHandler.UpdateTestEnv)
		r.Post("/config/testenv/{id}/delete", configHandler.DeleteTestEnv)

		// Test Env Scripts (Playwright)
		r.Post("/config/testenv/{id}/script/record", testEnvHandler.RecordScript)
		r.Get("/config/testenv/{id}/script/record/status", testEnvHandler.RecordStatus)
		r.Post("/config/testenv/{id}/script/save", testEnvHandler.SaveScript)
		r.Post("/config/testenv/{id}/script/run", testEnvHandler.RunScript)
		r.Get("/config/testenv/{id}/script/output", testEnvHandler.ScriptOutput)
	})

	// Callback API (no auth required, token-based verification)
	r.Post("/api/callback/build/{token}", callbackHandler.UploadArtifact)
	r.Post("/api/callback/build/{token}/status", callbackHandler.BuildStatus)
	r.Get("/api/callback/build/{token}/artifacts", callbackHandler.ListArtifacts)
	r.Get("/api/callback/build/{token}/artifacts/{artifactId}", callbackHandler.DownloadArtifact)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("[server] starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
