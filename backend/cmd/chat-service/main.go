package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/thehien0811/chatter/config"

	chatGRPC "github.com/thehien0811/chatter/internal/chat/transport/grpcserver"
	"github.com/thehien0811/chatter/internal/grpcserver"
	pb "github.com/thehien0811/chatter/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		log.Fatal(err)
	}
	defer db.Close()
	redisClient := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password:    cfg.Redis.Password,
		DB:          cfg.Redis.DB,
		DialTimeout: 5 * time.Second,
	})
	defer redisClient.Close()
	friendConn, err := grpc.NewClient(cfg.Services.FriendURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer friendConn.Close()
	var rabbitConnection *amqp.Connection
	var rabbitChannel *amqp.Channel
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		rabbitConnection, err = amqp.Dial(url)
		if err != nil {
			log.Printf("chat notification RabbitMQ unavailable; continuing without push notifications: %v", err)
		} else {
			defer rabbitConnection.Close()
			rabbitChannel, err = rabbitConnection.Channel()
			if err == nil {
				_, err = rabbitChannel.QueueDeclare("notification", true, false, false, false, nil)
			}
			if err != nil {
				log.Printf("chat notification queue setup failed; continuing without push notifications: %v", err)
				if rabbitChannel != nil {
					_ = rabbitChannel.Close()
				}
				rabbitChannel = nil
			}
		}
	}
	server := chatGRPC.New(db, redisClient, pb.NewFriendServiceClient(friendConn), rabbitChannel)
	if err := grpcserver.Serve("9003", func(g *grpc.Server) {
		pb.RegisterChatServiceServer(g, server)
	}); err != nil {
		log.Fatal(err)
	}
}
