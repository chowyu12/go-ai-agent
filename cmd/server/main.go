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
	"github.com/chowyu12/go-ai-agent/internal/config"
	"github.com/chowyu12/go-ai-agent/internal/handler"
	"github.com/chowyu12/go-ai-agent/internal/seed"
	"github.com/chowyu12/go-ai-agent/internal/store/mysql"
)

var configFile = flag.String("config", "etc/config.yaml", "config file path")

func main() {
	flag.Parse()

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetLevel(log.DebugLevel)

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.WithError(err).Fatal("load config failed")
	}

	store, err := mysql.New(cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("connect database failed")
	}
	defer store.Close()

	seed.Init(context.Background(), store)

	registry := agentpkg.NewToolRegistry()
	executor := agentpkg.NewExecutor(store, registry)

	mux := http.NewServeMux()

	handler.NewProviderHandler(store).Register(mux)
	handler.NewAgentHandler(store).Register(mux)
	handler.NewToolHandler(store).Register(mux)
	handler.NewSkillHandler(store).Register(mux)
	handler.NewChatHandler(store, executor).Register(mux)

	mountFrontend(mux)

	wrapped := handler.Logger(handler.CORS(mux))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      wrapped,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
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

func mountFrontend(mux *http.ServeMux) {
	distDir := "web/dist"
	if _, err := os.Stat(distDir); err != nil {
		return
	}
	frontendFS := http.FileServer(http.Dir(distDir))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			frontendFS.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(os.DirFS(distDir), path[1:]); err != nil {
			r.URL.Path = "/"
		}
		frontendFS.ServeHTTP(w, r)
	})
}
