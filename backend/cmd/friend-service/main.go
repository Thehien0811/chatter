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
	friendGRPC "github.com/thehien0811/chatter/internal/friend/transport/grpcserver"
	"github.com/thehien0811/chatter/internal/grpcserver"
	pb "github.com/thehien0811/chatter/proto"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr(cfg.Redis.Host, cfg.Redis.Port), Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	defer redisClient.Close()
	var rabbitChannel *amqp.Channel
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		conn, err := amqp.Dial(url)
		if err != nil {
			log.Printf("friend notification RabbitMQ unavailable; continuing with Redis: %v", err)
		} else {
			defer conn.Close()
			rabbitChannel, err = conn.Channel()
			if err == nil {
				err = rabbitChannel.ExchangeDeclare("friend.events", "topic", true, false, false, false, nil)
			}
			if err != nil {
				log.Printf("friend notification RabbitMQ setup failed; continuing with Redis: %v", err)
				if rabbitChannel != nil {
					_ = rabbitChannel.Close()
				}
				rabbitChannel = nil
			}
		}
	}
	server := friendGRPC.New(db, redisClient, rabbitChannel)
	defer server.Close()
	if err := grpcserver.Serve("9002", func(g *grpc.Server) { pb.RegisterFriendServiceServer(g, server) }); err != nil {
		log.Fatal(err)
	}
}

func redisAddr(host string, port int) string { return fmt.Sprintf("%s:%d", host, port) }
