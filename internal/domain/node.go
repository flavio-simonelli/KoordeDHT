package domain

// Node rappresenta un partecipante alla DHT
type Node struct {
	ID   ID     // identificatore nello spazio 2^b
	Addr string // indirizzo di rete, es. "127.0.0.1:5000"
}
