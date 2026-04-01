package checkinbot

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run() {
	cfg, err := ParseConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db := OpenDB(cfg.DBPath)
	defer db.Close()

	SeedAdmins(db, cfg.AdminID)

	bot, err := SetupBot(cfg, db)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go RunPinger(ctx, bot, db, cfg)
	go RunMissedMonitor(ctx, bot, db, cfg)

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: SetupHTTPServer(cfg, db, bot.Me.Username, spaHandler()),
	}
	go func() {
		log.Printf("web ui listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	go func() {
		log.Printf("bot started")
		bot.Start()
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	bot.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
