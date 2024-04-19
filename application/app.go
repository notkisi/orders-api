package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type App struct {
	router http.Handler
	rdb    *redis.Client
}

func New() *App {
	app := &App{
		rdb: redis.NewClient(&redis.Options{}),
	}

	app.loadRoutes()

	return app
}

func (a *App) Start(ctx context.Context) error {
	server := &http.Server{
		Addr:    ":3000",
		Handler: a.router,
	}

	err := a.rdb.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	defer func() {
		if err := a.rdb.Close(); err != nil {
			fmt.Println("Failed to close redis", err)
		}
	}()

	fmt.Println("Starting server")

	ch := make(chan error, 1)
	go func() {
		err = server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			ch <- err
		}
		close(ch)
	}()

	select {
	case err = <-ch:
		return err
	case <-ctx.Done():
		// cant use parent context as its channel Done is closed already at this point
		// init shutdown with max 10s
		timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fmt.Println("\nCTRL-C Caught. Initiating shutdown...")
		return server.Shutdown(timeout)
	}
}
