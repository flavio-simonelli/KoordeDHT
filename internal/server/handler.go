package server

import (
	pb "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/node"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Handler struct {
	pb.UnimplementedDHTServer
	node *node.Node
}

func (h *Handler) FindSuccessor(ctx context.Context, req *pb.FindSuccessorRequest) (*pb.FindSuccessorResponse, error) {
	targetID := req.GetId()
	successor, err := h.node.FindSuccessor(targetID)
	if err != nil {
		return nil, err
	}
	return &pb.FindSuccessorResponse{
		Node: &pb.Node{
			Id:      successor.ID,
			Address: successor.Addr,
		},
	}, nil
}

func (h *Handler) GetPredecessor(ctx context.Context, req *emptypb.Empty) (*pb.Node, error) {
	pred := h.node.GetPredecessor()
	return &pb.Node{
		Id:      pred.ID,
		Address: pred.Addr,
	}, nil
}

func (h *Handler) Notify(ctx context.Context, req *pb.Node) (*emptypb.Empty, error) {
	// controlla che la richiesta non sia nil e che il nodo sia davvero il mio nuovo predecessore
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "node cannot be nil")
	}
	newPred := domain.Node{
		ID:   domain.ID(req.Id),
		Addr: req.Address,
	}
	h.node.Notify(newPred)
	return &emptypb.Empty{}, nil
}

func (h *Handler) Ping(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
