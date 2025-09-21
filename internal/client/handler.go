package client

import (
	pb "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/domain"
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (cp *ClientPool) FindSuccessor(target domain.ID, serverAddr string) (domain.Node, error) {
	// recupera la connessione dal pool
	conn, err := cp.GetConn(serverAddr) //TODO: attenzione che il nodo di bootstrap non viene mai chiuso il suo client contection
	if err != nil {
		return domain.Node{}, err
	}
	// crea il client gRPC
	client := pb.NewDHTClient(conn)
	// context con timeout (es. 2 secondi)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) //TODO: modificare questa configurazione
	defer cancel()
	// invia la richiesta FindSuccessor
	resp, err := client.FindSuccessor(ctx, &pb.FindSuccessorRequest{Id: target})
	if err != nil {
		return domain.Node{}, err
	}
	// converte la risposta in domain.Node
	return domain.Node{
		ID:   resp.Node.Id,
		Addr: resp.Node.Address,
	}, nil
}

func (cp *ClientPool) GetPredecessor(serverAddr string) (domain.Node, error) {
	// recupera la connessione dal pool
	conn, err := cp.GetConn(serverAddr)
	if err != nil {
		return domain.Node{}, err
	}
	// crea il client gRPC
	client := pb.NewDHTClient(conn)
	// context con timeout (es. 2 secondi)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) //TODO: modificare questa configurazione
	defer cancel()
	// invia la richiesta GetPredecessor
	resp, err := client.GetPredecessor(ctx, &emptypb.Empty{})
	if err != nil {
		return domain.Node{}, err
	}
	// converte la risposta in domain.Node
	return domain.Node{
		ID:   resp.Id,
		Addr: resp.Address,
	}, nil
}

func (cp *ClientPool) Notify(self domain.Node, serverAddr string) error {
	// recupera la connessione dal pool
	conn, err := cp.GetConn(serverAddr)
	if err != nil {
		return err
	}
	// crea il client gRPC
	client := pb.NewDHTClient(conn)
	// context con timeout (es. 2 secondi)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) //TODO: modificare questa configurazione
	defer cancel()
	// invia la richiesta Notify
	_, err = client.Notify(ctx, &pb.Node{
		Id:      self.ID,
		Address: self.Addr,
	})
	return err
}

func (cp *ClientPool) Ping(serverAddr string) error {
	// recupera la connessione dal pool
	conn, err := cp.GetConn(serverAddr)
	if err != nil {
		return err
	}
	// crea il client gRPC
	client := pb.NewDHTClient(conn)
	// context con timeout (es. 2 secondi)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) //TODO: modificare questa configurazione
	defer cancel()
	// invia la richiesta Ping
	_, err = client.Ping(ctx, &emptypb.Empty{})
	return err
}
