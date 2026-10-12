package di

import (
	"log"
	"time"

	stationcmd "go-api/internal/application/command/station"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/messaging/rabbitmq"
	"go-api/internal/infrastructure/persistence/read"
	"go-api/internal/infrastructure/roulezeco"

	"gorm.io/gorm"
)

// Container wires dependencies for scheduled jobs.
type Container struct {
	PublishChangedHandler *stationcmd.PublishChangedHandler
	Conn                  *rabbitmq.Connection
	FeedInterval          time.Duration
}

// NewContainer opens RabbitMQ and builds the fuel-feed publisher.
func NewContainer(db *gorm.DB, env *config.Config) *Container {
	topology := rabbitmq.DefaultTopology(
		env.RabbitMQExchange,
		stationcmd.QueueName,
		stationcmd.RoutingKeys(),
		env.RabbitMQRetryTTLMS,
	)
	conn, err := rabbitmq.Connect(env.RabbitMQURL, topology)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}

	feedInterval := env.SchedulerFeedInterval
	if feedInterval <= 0 {
		feedInterval = 5 * time.Minute
	}

	return &Container{
		PublishChangedHandler: stationcmd.NewPublishChangedHandler(
			roulezeco.NewFeed(env.FuelFeedURL, nil),
			read.NewStationHashRepository(db),
			rabbitmq.NewPublisher(conn, env.RabbitMQExchange),
		),
		Conn:         conn,
		FeedInterval: feedInterval,
	}
}
