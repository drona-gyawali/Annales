package main

// cmd/benchmark/main.go
//
// Annales benchmark harness.
//
// Usage:
//   go run ./cmd/benchmark               # write + read throughput
//   BENCH_MODE=recovery go run ./cmd/benchmark  # restart-to-ready timing
//
// Every HTTP response is status-code validated. Operations that return a
// non-2xx status are counted as errors and excluded from throughput figures.
// The final summary prints both the success count and the error count so the
// reader can verify that reported throughput reflects only successful ops.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	baseURL    = "http://localhost:8080"
	totalOps   = 10_000
	valueSize  = 64 // bytes per value, fixed for reproducibility
	warmupOps  = 200
)

// ── helpers ──────────────────────────────────────────────────────────────────

func checkHealth(client *http.Client) bool {
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

func putKV(client *http.Client, key, value string) (int, error) {
	body, _ := json.Marshal(map[string]string{"value": value})
	req, err := http.NewRequest(http.MethodPut,
		baseURL+"/kv/"+key, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

func getKV(client *http.Client, key string) (int, error) {
	resp, err := client.Get(baseURL + "/kv/" + key)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

func fixedValue(n int) string {
	// deterministic value of exactly valueSize bytes
	s := fmt.Sprintf("val-%08d-", n)
	for len(s) < valueSize {
		s += "x"
	}
	return s[:valueSize]
}

// ── write benchmark ───────────────────────────────────────────────────────────

type writeResult struct {
	successes int64
	errors    int64
	elapsed   time.Duration
}

func runWrites(concurrency int) writeResult {
	client := &http.Client{Timeout: 30 * time.Second}

	// warm-up: not counted
	for i := 0; i < warmupOps; i++ {
		putKV(client, fmt.Sprintf("warmup-%d", i), fixedValue(i)) //nolint
	}

	var (
		successes int64
		errors    int64
		wg        sync.WaitGroup
		work      = make(chan int, totalOps)
	)

	for i := 0; i < totalOps; i++ {
		work <- i
	}
	close(work)

	start := time.Now()

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &http.Client{Timeout: 30 * time.Second}
			for i := range work {
				key := fmt.Sprintf("bench-write-%07d", i)
				status, err := putKV(c, key, fixedValue(i))
				if err != nil || status != http.StatusNoContent {
					atomic.AddInt64(&errors, 1)
				} else {
					atomic.AddInt64(&successes, 1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	return writeResult{
		successes: atomic.LoadInt64(&successes),
		errors:    atomic.LoadInt64(&errors),
		elapsed:   elapsed,
	}
}

// ── read benchmark ────────────────────────────────────────────────────────────

type readResult struct {
	latencies []float64 // milliseconds, successful ops only
	errors    int64
}

func runReads(n int) readResult {
	client := &http.Client{Timeout: 10 * time.Second}

	// seed a key we know exists
	putKV(client, "bench-read-target", fixedValue(0)) //nolint

	var (
		latencies []float64
		mu        sync.Mutex
		errors    int64
	)

	for i := 0; i < n; i++ {
		t0 := time.Now()
		status, err := getKV(client, "bench-read-target")
		latMs := float64(time.Since(t0).Microseconds()) / 1000.0

		if err != nil || status != http.StatusOK {
			atomic.AddInt64(&errors, 1)
			continue
		}
		mu.Lock()
		latencies = append(latencies, latMs)
		mu.Unlock()
	}

	return readResult{latencies: latencies, errors: errors}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	idx := int(math.Ceil(p/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// ── recovery benchmark ────────────────────────────────────────────────────────

func runRecovery() {
	// This mode is started BEFORE the server process is restarted.
	// It records the time from first poll until the server is both
	// healthy AND able to serve a previously committed key.
	//
	// Expected usage from the shell script:
	//   1. Write a known key to Annales.
	//   2. Stop the Annales container.
	//   3. Start BENCH_MODE=recovery go run ./cmd/benchmark  (this code).
	//   4. Restart the Annales container.
	//   5. This process measures time until health + data confirmed.

	client := &http.Client{Timeout: 2 * time.Second}

	// First, seed the known key while the server is still up (caller
	// should have done this, but we defensively skip if unreachable).
	knownKey := "recovery-probe"
	knownVal := fixedValue(42)
	putKV(client, knownKey, knownVal) //nolint — may fail if server already down

	fmt.Println("Recovery benchmark: waiting for Annales to restart...")
	start := time.Now()

	deadline := time.Now().Add(120 * time.Second)
	healthy := false
	for time.Now().Before(deadline) {
		if checkHealth(client) {
			healthy = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !healthy {
		fmt.Println("ERROR: Annales did not become healthy within 120 s")
		os.Exit(1)
	}
	healthyAt := time.Since(start)

	// Now verify the committed key is actually readable.
	status, err := getKV(client, knownKey)
	readyAt := time.Since(start)

	if err != nil || status != http.StatusOK {
		fmt.Printf(
			"ERROR: health OK at %v but key %q not readable (status=%d err=%v)\n",
			healthyAt, knownKey, status, err,
		)
		fmt.Println("Recovery measurement INVALID: server healthy but data not recovered.")
		os.Exit(1)
	}

	fmt.Printf("\n── Recovery timing ──────────────────────────────\n")
	fmt.Printf("  Health endpoint ready : %v\n", healthyAt.Round(time.Millisecond))
	fmt.Printf("  Committed key readable: %v\n", readyAt.Round(time.Millisecond))
	fmt.Printf("  (timer started before restart signal)\n")
	fmt.Printf("  Probe key             : %q\n", knownKey)
	fmt.Printf("  Status code           : %d (want 200)\n", status)
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	if os.Getenv("BENCH_MODE") == "recovery" {
		runRecovery()
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	if !checkHealth(client) {
		fmt.Fprintln(os.Stderr, "Annales is not reachable at", baseURL)
		os.Exit(1)
	}

	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println("  Annales Write Throughput Benchmark")
	fmt.Printf("  %d operations, value size %d bytes, %d warmup ops\n",
		totalOps, valueSize, warmupOps)
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Printf("  %-12s  %-12s  %-12s  %-10s  %-8s\n",
		"Concurrency", "Successes", "Errors", "Elapsed", "ops/s")
	fmt.Println("  ──────────────────────────────────────────────")

	for _, c := range []int{10, 50, 100} {
		r := runWrites(c)
		opsPerSec := 0.0
		if r.elapsed > 0 {
			opsPerSec = float64(r.successes) / r.elapsed.Seconds()
		}
		fmt.Printf("  %-12d  %-12d  %-12d  %-10s  %.0f\n",
			c, r.successes, r.errors,
			r.elapsed.Round(time.Millisecond),
			opsPerSec,
		)
		if r.errors > 0 {
			fmt.Printf("  WARNING: %d errors at concurrency %d — these are excluded from ops/s\n",
				r.errors, c)
		}
	}

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println("  Annales Cache-Hit Read Latency Benchmark")
	fmt.Printf("  %d sequential reads, single key (RAM-served)\n", totalOps)
	fmt.Println("══════════════════════════════════════════════════")

	for run := 1; run <= 2; run++ {
		r := runReads(totalOps)
		sort.Float64s(r.latencies)
		fmt.Printf("  Run %d — successful reads: %d  errors: %d\n",
			run, len(r.latencies), r.errors)
		if len(r.latencies) > 0 {
			fmt.Printf("    P50: %.3f ms  P95: %.3f ms  P99: %.3f ms\n",
				percentile(r.latencies, 50),
				percentile(r.latencies, 95),
				percentile(r.latencies, 99),
			)
		}
		if r.errors > 0 {
			fmt.Printf("  WARNING: %d read errors excluded from latency figures\n", r.errors)
		}
	}

	fmt.Println()
	fmt.Println("Benchmark complete.")
	fmt.Println("NOTE: ops/s counts ONLY responses with HTTP 204 (writes) or 200 (reads).")
	fmt.Println("      Errors are printed above and excluded from all reported figures.")
}