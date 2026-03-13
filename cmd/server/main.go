package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	agentpkg "github.com/chowyu12/go-ai-agent/internal/agent"
	"github.com/chowyu12/go-ai-agent/internal/auth"
	"github.com/chowyu12/go-ai-agent/internal/config"
	"github.com/chowyu12/go-ai-agent/internal/handler"
	"github.com/chowyu12/go-ai-agent/internal/seed"
	"github.com/chowyu12/go-ai-agent/internal/store/gormstore"
	"github.com/chowyu12/go-ai-agent/internal/tool/browser"
	"github.com/chowyu12/go-ai-agent/internal/workspace"
	"github.com/chowyu12/go-ai-agent/web"
)

var configFile = flag.String("config", "", "config file path (default: ~/.go-agent/config.yaml)")

func main() {
	flag.Parse()

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetLevel(log.DebugLevel)

	cfgPath := config.ConfigPath(*configFile)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.WithError(err).Fatal("load config failed")
	}
	log.WithField("path", cfgPath).Info("config loaded")

	if cfg.Log.Level != "" {
		if lvl, err := log.ParseLevel(cfg.Log.Level); err == nil {
			log.SetLevel(lvl)
			log.WithField("level", lvl).Info("log level configured")
		} else {
			log.WithFields(log.Fields{"level": cfg.Log.Level, "error": err}).Warn("invalid log level, using debug")
		}
	}

	if err := workspace.Init(cfg.Workspace); err != nil {
		log.WithError(err).Fatal("init workspace failed")
	}
	log.WithField("path", workspace.Root()).Info("workspace initialized")

	if cfg.Upload.Dir == "" || cfg.Upload.Dir == "./uploads" {
		cfg.Upload.Dir = workspace.Uploads()
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	if cfg.NeedsDatabaseSetup() {
		log.WithField("addr", addr).Warn("database not configured, starting setup wizard")
		log.Infof("→ please open http://localhost:%d in your browser to configure database", cfg.Server.Port)
		runSetupServer(cfg, cfgPath, addr)

		cfg, err = config.Load(cfgPath)
		if err != nil {
			log.WithError(err).Fatal("reload config after setup failed")
		}
		log.Info("database configured, continuing startup...")
	}

	store, err := gormstore.New(cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("connect database failed")
	}
	defer store.Close()

	seed.Init(context.Background(), store)

	if cfg.Browser.Visible {
		browser.SetVisible(true)
		log.Info("browser tool: visible mode enabled")
	}
	if cfg.Browser.Width > 0 && cfg.Browser.Height > 0 {
		browser.SetViewport(cfg.Browser.Width, cfg.Browser.Height)
	}
	if cfg.Browser.UserAgent != "" {
		browser.SetUserAgent(cfg.Browser.UserAgent)
	}
	if cfg.Browser.Proxy != "" {
		browser.SetProxy(cfg.Browser.Proxy)
	}

	registry := agentpkg.NewToolRegistry()
	executor := agentpkg.NewExecutor(store, registry)

	mux := http.NewServeMux()

	handler.NewAuthHandler(store, cfg.JWT.Secret, cfg.JWT.ExpireHours).Register(mux)
	handler.NewProviderHandler(store).Register(mux)
	handler.NewAgentHandler(store).Register(mux)
	handler.NewToolHandler(store).Register(mux)
	handler.NewSkillHandler(store).Register(mux)
	handler.NewMCPHandler(store).Register(mux)
	handler.NewChatHandler(store, executor).Register(mux)
	handler.NewFileHandler(store, cfg.Upload).Register(mux)

	mountFrontend(mux)

	authMW := auth.Middleware(auth.Config{
		JWTSecret:     []byte(cfg.JWT.Secret),
		TokenResolver: store,
	})
	wrapped := handler.Logger(handler.CORS(authMW(mux)))

	srv := &http.Server{
		Addr:        addr,
		Handler:     wrapped,
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		log.WithField("addr", addr).Info("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Error("server shutdown error")
	}
	log.Info("server stopped")
}

func runSetupServer(cfg *config.Config, cfgPath, addr string) {
	done := make(chan struct{})

	mux := http.NewServeMux()
	handler.NewSetupHandler(cfg, cfgPath, done).Register(mux)
	mountFrontend(mux)

	srv := &http.Server{
		Addr:        addr,
		Handler:     handler.Logger(handler.CORS(mux)),
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("setup server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-done:
		log.Info("database setup completed via web wizard")
	case <-quit:
		log.Info("setup interrupted, shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	signal.Stop(quit)
}

func mountFrontend(mux *http.ServeMux) {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.WithError(err).Warn("embedded frontend not available, skipping")
		return
	}
	fileServer := http.FileServer(http.FS(distFS))
	log.Info("serving embedded frontend")

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path != "/" {
			if _, err := fs.Stat(distFS, path[1:]); err != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}
