package main

import (
	"context"
	"log"

	"github.com/thehien0811/chatter/config"
	"github.com/thehien0811/chatter/internal/grpcserver"
	userRepo "github.com/thehien0811/chatter/internal/user/repository"
	userGRPC "github.com/thehien0811/chatter/internal/user/transport/grpcserver"
	userUseCase "github.com/thehien0811/chatter/internal/user/usecase"
	pb "github.com/thehien0811/chatter/proto"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	pool, err := userRepo.NewPostgresPool(context.Background(), cfg.DSN())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	service := userUseCase.NewUserUseCase(userRepo.NewUserRepository(pool))
	server := userGRPC.New(service)
	if err := grpcserver.Serve("9005", func(g *grpc.Server) {
		pb.RegisterUserServiceServer(g, server)
	}); err != nil {
		log.Fatal(err)
	}
}
