package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/notkisi/orders-api/application"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	app := application.New()
	err := app.Start(ctx)
	if err != nil {
		fmt.Println("failed to start app:", err)
	}
}
