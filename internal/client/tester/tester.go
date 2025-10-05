package tester

import (
	"KoordeDHT/internal/bootstrap"
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/client/tester/writer"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type Tester struct {
	cfg     *Config
	logger  logger.Logger
	writer  writer.Writer
	boot    bootstrap.Bootstrap
	space   domain.Space
	started time.Time
}

// New crea un nuovo tester
func New(cfg *Config, lgr logger.Logger, writer writer.Writer, boot bootstrap.Bootstrap, space domain.Space) *Tester {
	return &Tester{
		cfg:    cfg,
		logger: lgr,
		writer: writer,
		space:  space,
		boot:   boot,
	}
}

// Run esegue il test per la durata configurata o finché non viene interrotto
func (t *Tester) Run(ctx context.Context) error {
	t.logger.Info("Tester started", logger.F("duration", t.cfg.Simulation.Duration))
	t.started = time.Now()
	endTime := t.started.Add(t.cfg.Simulation.Duration)
	interval := time.Duration(float64(time.Second) / t.cfg.Query.Rate)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		now := time.Now()
		if now.After(endTime) {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := t.runQueryWave(ctx); err != nil {
				t.logger.Error("query wave failed", logger.F("err", err))
			}
		}
	}

	t.logger.Info("Tester finished")
	return nil
}

// runQueryWave esegue una “ondata” di lookup paralleli
func (t *Tester) runQueryWave(ctx context.Context) error {
	nodes, err := t.boot.Discover(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap discovery failed: %w", err)
	}
	if len(nodes) == 0 {
		t.logger.Warn("no nodes discovered")
		return nil
	}

	// Determina grado di parallelismo
	p := randomInt(t.cfg.Query.Parallelism.MinWorkers, t.cfg.Query.Parallelism.MaxWorkers)
	t.logger.Info("Starting query wave",
		logger.F("parallel", p),
		logger.F("nodes", len(nodes)),
	)

	var wg sync.WaitGroup
	wg.Add(p)

	for i := 0; i < p; i++ {
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
				t.doLookup(nodes)
			}
		}()
	}

	wg.Wait()
	return nil
}

// doLookup seleziona un nodo random e simula una lookup
func (t *Tester) doLookup(nodes []string) {
	node := nodes[rand.Intn(len(nodes))]
	key, err := t.generateRandomID()
	if err != nil {
		t.logger.Warn("failed to generate random ID", logger.F("err", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c, conn, err := client.Connect(node)
	if err != nil {
		t.logger.Warn("failed to connect to node", logger.F("node", node), logger.F("err", err))
		return
	}
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			t.logger.Warn("failed to close connection", logger.F("node", node), logger.F("err", err))
		}
	}(conn)

	// Esegui il lookup
	_, delay, err := client.Lookup(ctx, c, key)
	result := "SUCCESS"
	if err != nil {
		result = "ERROR_" + err.Error()
	}

	// logga il risultato
	t.logger.Info("Lookup result",
		logger.F("node", node),
		logger.F("key", key),
		logger.F("result", result),
		logger.F("delay_ms", delay.Milliseconds()),
	)

	// scrivi il risultato nel CSV
	if err := t.writer.WriteRow(node, result, delay); err != nil {
		t.logger.Warn("failed to write CSV row", logger.F("err", err))
	}
}

// randomInt restituisce un numero intero random compreso tra min e max inclusi
func randomInt(min, max int) int {
	if min >= max {
		return min
	}
	return rand.Intn(max-min+1) + min
}

// generateRandomID usa il metodo domain.Space.NewIdFromString()
// con un input casuale, così ottieni un ID valido nel tuo spazio.
func (t *Tester) generateRandomID() (string, error) {
	// genera 16 byte casuali per la stringa sorgente
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate random input: %w", err)
	}
	randomStr := hex.EncodeToString(buf)

	// usa la logica di domain.Space per ottenere un ID valido
	id := t.space.NewIdFromString(randomStr)
	idString := id.ToHexString(true)
	return idString, nil
}
