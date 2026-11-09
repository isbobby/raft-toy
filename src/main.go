package main

import (
	"log"
	"net"
	"raft-toy/internal/raft"
	pb "raft-toy/pb/raft"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()

	raftService := raft.NewService()

	pb.RegisterRaftServiceServer(grpcServer, raftService)

	reflection.Register(grpcServer)

	log.Println("Raft gRPC server listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
