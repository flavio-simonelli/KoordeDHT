package tester

import (
	"KoordeDHT/internal/bootstrap"
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/client/tester/writer"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"encoding/hex"
	"errors"
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

// New create a new Tester instance
func New(cfg *Config, lgr logger.Logger, writer writer.Writer, boot bootstrap.Bootstrap, space domain.Space) *Tester {
	return &Tester{
		cfg:    cfg,
		logger: lgr,
		writer: writer,
		space:  space,
		boot:   boot,
	}
}

// Run starts the tester for the configured duration or until the context is cancelled
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

// runQueryWave executes a wave of parallel queries
func (t *Tester) runQueryWave(ctx context.Context) error {
	nodes, err := t.boot.Discover(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap discovery failed: %w", err)
	}
	if len(nodes) == 0 {
		t.logger.Warn("no nodes discovered")
		return nil
	}

	// choise a random number of parallel workers between min and max
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

// doLookup performs a single lookup operation on a random node
func (t *Tester) doLookup(nodes []string) {
	node := nodes[rand.Intn(len(nodes))]
	key, err := t.generateRandomID()
	if err != nil {
		t.logger.Warn("failed to generate random ID", logger.F("err", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.cfg.Query.Timeout)
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

	_, delay, err := client.Lookup(ctx, c, key)
	var result string
	if err != nil {
		switch {
		case errors.Is(err, client.ErrUnavailable):
			// Node not reachable, skip writing to CSV
			t.logger.Debug("node unavailable (skipping CSV)",
				logger.F("node", node),
				logger.F("id", key),
				logger.F("err", err),
			)
			return

		case errors.Is(err, client.ErrDeadlineExceeded):
			result = "TIMEOUT"

		case errors.Is(err, client.ErrNotFound):
			result = "NOT_FOUND"

		default:
			result = fmt.Sprintf("ERROR_%v", err)
		}
	} else {
		result = "SUCCESS"
	}

	// log the result
	t.logger.Info("Lookup result",
		logger.F("node", node),
		logger.F("key", key),
		logger.F("result", result),
		logger.F("delay_ms", delay.Milliseconds()),
	)

	// write to CSV
	if err := t.writer.WriteRow(node, result, delay); err != nil {
		t.logger.Warn("failed to write CSV row", logger.F("err", err))
	}
}

// randomInt returns a random integer between min and max (inclusive)
func randomInt(min, max int) int {
	if min >= max {
		return min
	}
	return rand.Intn(max-min+1) + min
}

// generateRandomID generates a random valid ID string using the domain.Space logic
func (t *Tester) generateRandomID() (string, error) {
	// create a random byte slice
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate random input: %w", err)
	}
	randomStr := hex.EncodeToString(buf)

	// convert to ID using domain.Space
	id := t.space.NewIdFromString(randomStr)
	idString := id.ToHexString(true)
	return idString, nil
}
