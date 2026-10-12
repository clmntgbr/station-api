package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go-api/cmd/scheduler/di"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/persistence/schema"
)

func main() {
	env := config.Load()
	db := config.ConnectDatabase(env)

	if err := schema.AssertModelsMatchDB(db); err != nil {
		log.Fatalf("schema check failed: %v", err)
	}

	container := di.NewContainer(db, env)
	defer container.Conn.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	// Register periodic jobs here. Each runs on its own clock-aligned loop.
	wg.Add(1)
	go func() {
		defer wg.Done()
		runJob(ctx, "fuel_feed", container.FeedInterval, func(jobCtx context.Context) {
			runFuelFeed(jobCtx, container)
		})
	}()

	<-ctx.Done()
	wg.Wait()
	log.Println("scheduler stopped")
}

var fuelFeedRunning atomic.Bool

func runFuelFeed(ctx context.Context, container *di.Container) {
	if !fuelFeedRunning.CompareAndSwap(false, true) {
		log.Println("scheduler: fuel_feed skipped (previous run still in progress)")
		return
	}
	defer fuelFeedRunning.Store(false)

	startedAt := time.Now().UTC()
	log.Println("scheduler: fuel_feed pulling...")
	result, err := container.PublishChangedHandler.Handle(ctx)
	if err != nil {
		log.Printf("scheduler: fuel_feed failed: %v", err)
		return
	}
	log.Printf(
		"scheduler: fuel_feed done stations=%d created=%d info=%d prices=%d unchanged=%d duration=%s",
		result.Stations,
		result.Created,
		result.Info,
		result.Prices,
		result.Unchanged,
		time.Since(startedAt).Round(time.Millisecond),
	)
}
