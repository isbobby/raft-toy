package raft

import (
	"context"
	pb "raft-toy/pb/raft"
)

type service struct {
	pb.UnimplementedRaftServiceServer
}

func (s *service) RequestVote(ctx context.Context, req *pb.RequestVoteRequest) (*pb.RequestVoteResponse, error) {
	return &pb.RequestVoteResponse{}, nil
}

func (s *service) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	return &pb.AppendEntriesResponse{}, nil
}

func (s *service) InstallSnapshot(ctx context.Context, req *pb.InstallSnapshotRequest) (*pb.InstallSnapshotResponse, error) {
	return &pb.InstallSnapshotResponse{}, nil
}

func NewService() *service {
	return &service{}
}
