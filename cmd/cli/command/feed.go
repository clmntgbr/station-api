package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	stationcmd "go-api/internal/application/command/station"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/messaging/rabbitmq"
	"go-api/internal/infrastructure/persistence/read"
	"go-api/internal/infrastructure/roulezeco"

	"github.com/spf13/cobra"
)

func NewFeedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feed",
		Short: "Fuel feed commands",
	}
	cmd.AddCommand(newFeedPullCommand())
	return cmd
}

func newFeedPullCommand() *cobra.Command {
	var feedURL string

	cmd := &cobra.Command{
		Use:          "pull",
		Short:        "Download the fuel feed and publish stations whose hash changed",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			return runFeedPull(ctx, feedURL, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&feedURL, "url", "", "Fuel feed archive URL")
	return cmd
}

func runFeedPull(ctx context.Context, feedURL string, out io.Writer) error {
	env := config.Load()
	db := config.ConnectDatabase(env)
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	defer sqlDB.Close()

	topology := rabbitmq.DefaultTopology(
		env.RabbitMQExchange,
		stationcmd.QueueName,
		stationcmd.RoutingKeys(),
		env.RabbitMQRetryTTLMS,
	)
	conn, err := rabbitmq.Connect(env.RabbitMQURL, topology)
	if err != nil {
		return err
	}
	defer conn.Close()

	handler := stationcmd.NewPublishChangedHandler(
		roulezeco.NewFeed(feedURL, nil),
		read.NewStationHashRepository(db),
		rabbitmq.NewPublisher(conn, env.RabbitMQExchange),
	)
	handler.OnProgress(func(read int) {
		fmt.Fprintf(out, "  %d\n", read)
	})

	fmt.Fprintln(out, "Downloading fuel feed...")
	started := time.Now()
	result, err := handler.Handle(ctx)
	if err != nil {
		return err
	}
	writeFeedSummary(out, result, time.Since(started).Round(time.Millisecond))
	return nil
}

func writeFeedSummary(out io.Writer, result stationcmd.Result, elapsed time.Duration) {
	fmt.Fprintf(out, "stations: %d\n", result.Stations)
	fmt.Fprintf(out, "created: %d\n", result.Created)
	fmt.Fprintf(out, "info: %d\n", result.Info)
	fmt.Fprintf(out, "prices: %d\n", result.Prices)
	fmt.Fprintf(out, "unchanged: %d\n", result.Unchanged)
	fmt.Fprintf(out, "elapsed: %s\n", elapsed)
}
