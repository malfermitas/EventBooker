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
	"eventbooker/internal/integration/notifier"
	"eventbooker/internal/logging"
	"eventbooker/internal/repository/postgres"
	"eventbooker/internal/service"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
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

	appLogger, err := logging.NewEventBookerLogger(cfg.Name, cfg.Env, cfg.Logger.Engine, cfg.Logger.Level)
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

	userRepository := postgres.NewUserRepository(appLogger, db)
	eventRepository := postgres.NewEventRepository(appLogger, db)
	bookingRepository := postgres.NewBookingRepository(appLogger, db)
	refreshTokenRepository := postgres.NewRefreshTokenRepository(appLogger, db)
	notifierClient := notifier.NewClient(cfg.Notifier, appLogger)

	eventService := service.NewEventService(
		appLogger,
		notifierClient,
		postgres.NewTxManager(txManager),
		userRepository,
		eventRepository,
		bookingRepository,
	)
	authService := service.NewAuthService(
		appLogger,
		notifierClient,
		postgres.NewTxManager(txManager),
		userRepository,
		refreshTokenRepository,
		cfg.Auth,
	)

	authHandler := handler.NewAuthHandler(appLogger, authService, cfg.Auth)
	eventHandler := handler.NewEventHandler(appLogger, eventService)
	frontendHandler := handler.NewFrontendHandler(cfg.Notifier.TelegramBotUsername)
	authMiddleware := transportmiddleware.NewAuthMiddleware(appLogger, authService)
	router := deliveryhttp.NewRouter(authHandler, eventHandler, frontendHandler, authMiddleware)

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
			appLogger.Errorw("http server stopped with error", "error", serveErr)
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
