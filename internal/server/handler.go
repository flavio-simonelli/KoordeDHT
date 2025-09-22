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
	targetID := req.TargetID
	currentI := req.CurrentI
	kshift := req.Kshift
	var successor domain.Node
	var err error
	// Caso INIT: currentI e kshift vuoti
	if len(req.CurrentI) == 0 && len(req.Kshift) == 0 {
		successor, err = h.node.FindSuccessorInit(targetID)
	} else {
		// Caso normale
		successor, err = h.node.FindSuccessor(targetID, currentI, kshift)
	}
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

func (h *Handler) FindPredecessor(ctx context.Context, req *pb.FindSuccessorRequest) (*pb.FindSuccessorResponse, error) {
	targetID := req.TargetID
	currentI := req.CurrentI
	kshift := req.Kshift
	var predecessor domain.Node
	var err error
	// Caso INIT: currentI e kshift vuoti
	if len(req.CurrentI) == 0 && len(req.Kshift) == 0 {
		predecessor, err = h.node.FindPredecessorInit(targetID)
	} else {
		// Caso normale
		predecessor, err = h.node.FindPredecessor(targetID, currentI, kshift)
	}
	if err != nil {
		return nil, err
	}
	return &pb.FindSuccessorResponse{
		Node: &pb.Node{
			Id:      predecessor.ID,
			Address: predecessor.Addr,
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

func (h *Handler) GetSuccessor(ctx context.Context, req *emptypb.Empty) (*pb.Node, error) {
	succ := h.node.GetSuccessor()
	return &pb.Node{
		Id:      succ.ID,
		Address: succ.Addr,
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

func (h *Handler) Put(ctx context.Context, req *pb.PutRequest) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	err := h.node.Put(domain.Resource{Key: req.GetKey(), Value: req.GetValue()})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	res, err := h.node.Get(req.GetKey())
	if err != nil {
		return nil, err
	}
	return &pb.GetResponse{
		Value: res.Value,
	}, nil
}
