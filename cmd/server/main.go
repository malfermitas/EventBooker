package main

import (
	"context"
	"errors"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"

	appconfig "eventbooker/internal/config"
	deliveryhttp "eventbooker/internal/delivery/http"
	"eventbooker/internal/delivery/http/handler"
	transportmiddleware "eventbooker/internal/delivery/http/middleware"
	"eventbooker/internal/repository/postgres"
	"eventbooker/internal/service"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
	"github.com/wb-go/wbf/logger"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	cfg, err := appconfig.LoadAppConfig()
	if err != nil {
		return err
	}

	appLogger, err := logger.InitLogger(
		parseLoggerEngine(cfg.Logger.Engine),
		cfg.Name,
		cfg.Env,
		logger.WithLevel(parseLogLevel(cfg.Logger.Level)),
	)
	if err != nil {
		return err
	}

	db, err := pgxdriver.New(
		cfg.Postgres.DSN,
		appLogger,
		pgxdriver.MaxPoolSize(cfg.Postgres.MaxPoolSize),
		pgxdriver.MaxConnAttempts(cfg.Postgres.ConnAttempts),
	)
	if err != nil {
		return err
	}
	defer db.Close()

	if err = db.Ping(context.Background()); err != nil {
		return err
	}

	txManager, err := transaction.NewManager(
		db,
		appLogger,
		transaction.MaxAttempts(cfg.Transaction.MaxAttempts),
		transaction.BaseRetryDelay(cfg.Transaction.BaseRetryDelay()),
		transaction.MaxRetryDelay(cfg.Transaction.MaxRetryDelay()),
	)
	if err != nil {
		return err
	}

	userRepository := postgres.NewUserRepository(db)
	eventRepository := postgres.NewEventRepository(db)
	bookingRepository := postgres.NewBookingRepository(db)
	refreshTokenRepository := postgres.NewRefreshTokenRepository(db)

	eventService := service.NewEventService(
		postgres.NewTxManager(txManager),
		userRepository,
		eventRepository,
		bookingRepository,
	)
	authService := service.NewAuthService(
		postgres.NewTxManager(txManager),
		userRepository,
		refreshTokenRepository,
		cfg.Auth,
	)

	authHandler := handler.NewAuthHandler(authService, cfg.Auth)
	eventHandler := handler.NewEventHandler(eventService)
	authMiddleware := transportmiddleware.NewAuthMiddleware(authService)
	router := deliveryhttp.NewRouter(authHandler, eventHandler, authMiddleware)

	server := &stdhttp.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout(),
		WriteTimeout: cfg.HTTP.WriteTimeout(),
		IdleTimeout:  cfg.HTTP.IdleTimeout(),
	}

	errCh := make(chan error, 1)
	go func() {
		appLogger.Infow("starting http server", "addr", server.Addr)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, stdhttp.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case serveErr := <-errCh:
		return serveErr
	case sig := <-signalCh:
		appLogger.Infow("shutdown signal received", "signal", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout())
	defer cancel()

	appLogger.Info("shutting down http server")
	return server.Shutdown(shutdownCtx)
}

func parseLoggerEngine(value string) logger.Engine {
	switch value {
	case string(logger.ZapEngine):
		return logger.ZapEngine
	case string(logger.ZerologEngine):
		return logger.ZerologEngine
	case string(logger.LogrusEngine):
		return logger.LogrusEngine
	default:
		return logger.SlogEngine
	}
}

func parseLogLevel(value string) logger.Level {
	switch value {
	case "debug":
		return logger.DebugLevel
	case "warn":
		return logger.WarnLevel
	case "error":
		return logger.ErrorLevel
	default:
		return logger.InfoLevel
	}
}
