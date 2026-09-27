package grpcserver

import pb "github.com/thehien0811/chatter/proto"

type Server struct {
	pb.UnimplementedChatServiceServer
}

func New() *Server {
	return &Server{}
}
