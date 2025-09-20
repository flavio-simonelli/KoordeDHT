package server

import (
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/node"
)

type Handler struct {
	dhtv1.UnimplementedDHTServer
	node *node.Node
}
