package node

import "KoordeDHT/internal/routingtable"

type Node struct {
	rt *routingtable.RoutingTable
}

func New(rt *routingtable.RoutingTable) *Node {
	return &Node{rt: rt}
}
