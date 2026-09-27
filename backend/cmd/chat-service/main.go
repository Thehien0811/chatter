package main

import (
	"log"

	chatGRPC "github.com/thehien0811/chatter/internal/chat/transport/grpcserver"
	"github.com/thehien0811/chatter/internal/grpcserver"
	pb "github.com/thehien0811/chatter/proto"
	"google.golang.org/grpc"
)

func main() {
	server := chatGRPC.New()
	if err := grpcserver.Serve("9003", func(g *grpc.Server) {
		pb.RegisterChatServiceServer(g, server)
	}); err != nil {
		log.Fatal(err)
	}
}
