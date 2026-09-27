package main

import (
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thehien0811/chatter/config"
	authapp "github.com/thehien0811/chatter/internal/auth/app"
	authGRPC "github.com/thehien0811/chatter/internal/auth/transport/grpcserver"
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
	secret := cfg.JWT.Secret
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	target := strings.TrimPrefix(strings.TrimPrefix(cfg.Services.UserURL, "http://"), "https://")
	userConn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial user service: %v", err)
	}
	defer userConn.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     net.JoinHostPort(cfg.Redis.Host, strconv.Itoa(cfg.Redis.Port)),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()
	jwt := authapp.NewJWTService(secret)
	refresh := authapp.NewRefreshTokenManager(redisClient, secret, 7*24*time.Hour)
	server := authGRPC.New(pb.NewUserServiceClient(userConn), jwt, refresh, cfg.JWT.Expiry)
	if err := grpcserver.Serve("9001", func(g *grpc.Server) {
		pb.RegisterAuthServiceServer(g, server)
	}); err != nil {
		log.Fatal(err)
	}
}
