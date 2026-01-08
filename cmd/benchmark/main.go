// Benchmark tool for testing Osprey against PaySim fraud data.
//
// Usage:
//   go run cmd/benchmark/main.go -csv /path/to/paysim.csv -url http://localhost:8080
//
// This tool:
//   1. Reads PaySim transaction data (with fraud labels)
//   2. Sends each transaction to Osprey for evaluation
//   3. Compares Osprey's verdict (ALRT/NALT) with actual fraud labels
//   4. Calculates precision, recall, F1-score, and confusion matrix
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// PaySimTransaction represents a row from the PaySim dataset
type PaySimTransaction struct {
	Step           int
	Type           string
	Amount         float64
	NameOrig       string
	OldBalanceOrg  float64
	NewBalanceOrig float64
	NameDest       string
	OldBalanceDest float64
	NewBalanceDest float64
	IsFraud        bool
	IsFlaggedFraud bool
}

// EvaluateRequest is the Osprey API request format
type EvaluateRequest struct {
	Type     string         `json:"type"`
	Debtor   Party          `json:"debtor"`
	Creditor Party          `json:"creditor"`
	Amount   Amount         `json:"amount"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Party struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
}

type Amount struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

// EvaluateResponse is the Osprey API response format
type EvaluateResponse struct {
	EvaluationID string   `json:"evaluationId"`
	Status       string   `json:"status"` // "ALRT" or "NALT"
	Score        float64  `json:"score"`
	Reasons      []string `json:"reasons"`
}

// Metrics tracks benchmark results
type Metrics struct {
	TruePositives  int64 // Fraud detected as ALRT
	FalsePositives int64 // Non-fraud detected as ALRT
	TrueNegatives  int64 // Non-fraud detected as NALT
	FalseNegatives int64 // Fraud detected as NALT (missed fraud!)

	TotalProcessed int64
	TotalFraud     int64
	TotalNonFraud  int64
	TotalErrors    int64

	ProcessingTimeMs int64
}

func main() {
	// Parse flags
	csvPath := flag.String("csv", "", "Path to PaySim CSV file")
	baseURL := flag.String("url", "http://localhost:8080", "Osprey base URL")
	tenantID := flag.String("tenant", "benchmark-test", "Tenant ID for requests")
	limit := flag.Int("limit", 10000, "Maximum transactions to process (0 = all)")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	fraudOnly := flag.Bool("fraud-only", false, "Only test fraud transactions")
	sampleRate := flag.Float64("sample", 1.0, "Sample rate for non-fraud (0.0-1.0)")
	verbose := flag.Bool("verbose", false, "Print each transaction result")
	flag.Parse()

	if *csvPath == "" {
		fmt.Println("Usage: benchmark -csv /path/to/paysim.csv [-url http://localhost:8080]")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          OSPREY BENCHMARK - PaySim Fraud Detection            ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Printf("\nCSV File:    %s\n", *csvPath)
	fmt.Printf("Osprey URL:  %s\n", *baseURL)
	fmt.Printf("Tenant ID:   %s\n", *tenantID)
	fmt.Printf("Workers:     %d\n", *workers)
	fmt.Printf("Limit:       %d\n", *limit)
	fmt.Printf("Fraud Only:  %v\n", *fraudOnly)
	fmt.Printf("Sample Rate: %.2f\n", *sampleRate)
	fmt.Println()

	// Check Osprey is running
	if err := checkHealth(*baseURL); err != nil {
		fmt.Printf("ERROR: Osprey not reachable at %s: %v\n", *baseURL, err)
		fmt.Println("\nMake sure Osprey is running:")
		fmt.Println("  cd osprey && go run cmd/osprey/main.go")
		os.Exit(1)
	}
	fmt.Println("✓ Osprey is healthy")

	// Read PaySim data
	fmt.Printf("\nReading PaySim data from %s...\n", *csvPath)
	transactions, err := readPaySimCSV(*csvPath, *limit, *fraudOnly, *sampleRate)
	if err != nil {
		fmt.Printf("ERROR: Failed to read CSV: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Loaded %d transactions\n", len(transactions))

	// Count fraud vs non-fraud
	fraudCount := 0
	for _, tx := range transactions {
		if tx.IsFraud {
			fraudCount++
		}
	}
	fmt.Printf("  - Fraud:     %d (%.2f%%)\n", fraudCount, 100*float64(fraudCount)/float64(len(transactions)))
	fmt.Printf("  - Non-fraud: %d (%.2f%%)\n", len(transactions)-fraudCount, 100*float64(len(transactions)-fraudCount)/float64(len(transactions)))

	// Run benchmark
	fmt.Printf("\nRunning benchmark with %d workers...\n", *workers)
	startTime := time.Now()
	metrics := runBenchmark(transactions, *baseURL, *tenantID, *workers, *verbose)
	duration := time.Since(startTime)

	// Print results
	printResults(metrics, duration)
}

func checkHealth(baseURL string) error {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

func readPaySimCSV(path string, limit int, fraudOnly bool, sampleRate float64) ([]PaySimTransaction, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Map column indices
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[strings.ToLower(col)] = i
	}

	var transactions []PaySimTransaction
	sampleCounter := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip malformed rows
		}

		isFraud := record[colIndex["isfraud"]] == "1"

		// Apply filters
		if fraudOnly && !isFraud {
			continue
		}

		// Sample non-fraud transactions
		if !isFraud && sampleRate < 1.0 {
			sampleCounter++
			if float64(sampleCounter%100)/100.0 >= sampleRate {
				continue
			}
		}

		step, _ := strconv.Atoi(record[colIndex["step"]])
		amount, _ := strconv.ParseFloat(record[colIndex["amount"]], 64)
		oldBalanceOrg, _ := strconv.ParseFloat(record[colIndex["oldbalanceorg"]], 64)
		newBalanceOrig, _ := strconv.ParseFloat(record[colIndex["newbalanceorig"]], 64)
		oldBalanceDest, _ := strconv.ParseFloat(record[colIndex["oldbalancedest"]], 64)
		newBalanceDest, _ := strconv.ParseFloat(record[colIndex["newbalancedest"]], 64)
		isFlaggedFraud := record[colIndex["isflaggedfraud"]] == "1"

		tx := PaySimTransaction{
			Step:           step,
			Type:           record[colIndex["type"]],
			Amount:         amount,
			NameOrig:       record[colIndex["nameorig"]],
			OldBalanceOrg:  oldBalanceOrg,
			NewBalanceOrig: newBalanceOrig,
			NameDest:       record[colIndex["namedest"]],
			OldBalanceDest: oldBalanceDest,
			NewBalanceDest: newBalanceDest,
			IsFraud:        isFraud,
			IsFlaggedFraud: isFlaggedFraud,
		}

		transactions = append(transactions, tx)

		if limit > 0 && len(transactions) >= limit {
			break
		}
	}

	return transactions, nil
}

func runBenchmark(transactions []PaySimTransaction, baseURL, tenantID string, numWorkers int, verbose bool) *Metrics {
	metrics := &Metrics{}

	// Create work channel
	work := make(chan PaySimTransaction, 100)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 10 * time.Second}

			for tx := range work {
				start := time.Now()
				result, err := evaluateTransaction(client, baseURL, tenantID, tx)
				elapsed := time.Since(start).Milliseconds()

				atomic.AddInt64(&metrics.ProcessingTimeMs, elapsed)
				atomic.AddInt64(&metrics.TotalProcessed, 1)

				if err != nil {
					atomic.AddInt64(&metrics.TotalErrors, 1)
					if verbose {
						fmt.Printf("ERROR: %s -> %v\n", tx.NameOrig, err)
					}
					continue
				}

				// Track actual labels
				if tx.IsFraud {
					atomic.AddInt64(&metrics.TotalFraud, 1)
				} else {
					atomic.AddInt64(&metrics.TotalNonFraud, 1)
				}

				// Calculate confusion matrix
				predicted := result.Status == "ALRT"
				actual := tx.IsFraud

				if predicted && actual {
					atomic.AddInt64(&metrics.TruePositives, 1)
				} else if predicted && !actual {
					atomic.AddInt64(&metrics.FalsePositives, 1)
				} else if !predicted && !actual {
					atomic.AddInt64(&metrics.TrueNegatives, 1)
				} else { // !predicted && actual
					atomic.AddInt64(&metrics.FalseNegatives, 1)
				}

				if verbose {
					status := "✓"
					if (predicted && !actual) || (!predicted && actual) {
						status = "✗"
					}
					name := tx.NameOrig
					if len(name) > 10 {
						name = name[:10]
					}
					fmt.Printf("%s %-10s | Type: %-8s | Amount: $%12.2f | Fraud: %-5v | Osprey: %-4s (%.2f) | Drain: %v\n",
						status,
						name,
						tx.Type,
						tx.Amount,
						tx.IsFraud,
						result.Status,
						result.Score,
						tx.NewBalanceOrig == 0 && tx.OldBalanceOrg > 0,
					)
				}
			}
		}()
	}

	// Send work
	for _, tx := range transactions {
		work <- tx
	}
	close(work)

	// Wait for completion
	wg.Wait()

	return metrics
}

func evaluateTransaction(client *http.Client, baseURL, tenantID string, tx PaySimTransaction) (*EvaluateResponse, error) {
	// Build request matching Osprey's expected format
	req := EvaluateRequest{
		Type: tx.Type,
		Debtor: Party{
			ID:        tx.NameOrig,
			AccountID: tx.NameOrig + "-acc",
		},
		Creditor: Party{
			ID:        tx.NameDest,
			AccountID: tx.NameDest + "-acc",
		},
		Amount: Amount{
			Value:    tx.Amount,
			Currency: "USD",
		},
		// Pass balance data for AccountDrainRule
		Metadata: map[string]any{
			"old_balance": tx.OldBalanceOrg,
			"new_balance": tx.NewBalanceOrig,
			"step":        tx.Step,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, baseURL+"/evaluate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result EvaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func printResults(m *Metrics, duration time.Duration) {
	fmt.Println("\n╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      BENCHMARK RESULTS                        ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")

	fmt.Printf("\n📊 DATASET STATISTICS\n")
	fmt.Printf("   Total Processed:  %d\n", m.TotalProcessed)
	fmt.Printf("   Total Fraud:      %d\n", m.TotalFraud)
	fmt.Printf("   Total Non-Fraud:  %d\n", m.TotalNonFraud)
	fmt.Printf("   Errors:           %d\n", m.TotalErrors)

	fmt.Printf("\n📈 CONFUSION MATRIX\n")
	fmt.Println("                        Predicted")
	fmt.Println("                    ALRT        NALT")
	fmt.Println("              ┌──────────┬──────────┐")
	fmt.Printf("   Actual  F  │ %8d │ %8d │  (TP, FN)\n", m.TruePositives, m.FalseNegatives)
	fmt.Println("              ├──────────┼──────────┤")
	fmt.Printf("          NF  │ %8d │ %8d │  (FP, TN)\n", m.FalsePositives, m.TrueNegatives)
	fmt.Println("              └──────────┴──────────┘")

	// Calculate metrics
	precision := float64(0)
	if m.TruePositives+m.FalsePositives > 0 {
		precision = float64(m.TruePositives) / float64(m.TruePositives+m.FalsePositives)
	}

	recall := float64(0)
	if m.TruePositives+m.FalseNegatives > 0 {
		recall = float64(m.TruePositives) / float64(m.TruePositives+m.FalseNegatives)
	}

	f1 := float64(0)
	if precision+recall > 0 {
		f1 = 2 * (precision * recall) / (precision + recall)
	}

	accuracy := float64(0)
	total := m.TruePositives + m.TrueNegatives + m.FalsePositives + m.FalseNegatives
	if total > 0 {
		accuracy = float64(m.TruePositives+m.TrueNegatives) / float64(total)
	}

	fmt.Printf("\n🎯 DETECTION METRICS\n")
	fmt.Printf("   Precision:  %.4f  (of alerts, how many were actual fraud)\n", precision)
	fmt.Printf("   Recall:     %.4f  (of fraud, how many did we catch)\n", recall)
	fmt.Printf("   F1-Score:   %.4f  (harmonic mean of precision & recall)\n", f1)
	fmt.Printf("   Accuracy:   %.4f  (overall correct predictions)\n", accuracy)

	// Detection rate analysis
	fmt.Printf("\n🔍 DETECTION ANALYSIS\n")
	if m.TotalFraud > 0 {
		detectionRate := float64(m.TruePositives) / float64(m.TotalFraud) * 100
		missRate := float64(m.FalseNegatives) / float64(m.TotalFraud) * 100
		fmt.Printf("   Fraud Detected:    %d / %d (%.2f%%)\n", m.TruePositives, m.TotalFraud, detectionRate)
		fmt.Printf("   Fraud Missed:      %d / %d (%.2f%%) ⚠️\n", m.FalseNegatives, m.TotalFraud, missRate)
	}
	if m.TotalNonFraud > 0 {
		falseAlarmRate := float64(m.FalsePositives) / float64(m.TotalNonFraud) * 100
		fmt.Printf("   False Alarms:      %d / %d (%.2f%%)\n", m.FalsePositives, m.TotalNonFraud, falseAlarmRate)
	}

	fmt.Printf("\n⏱️  PERFORMANCE\n")
	fmt.Printf("   Total Duration:   %v\n", duration.Round(time.Millisecond))
	if m.TotalProcessed > 0 {
		avgMs := float64(m.ProcessingTimeMs) / float64(m.TotalProcessed)
		tps := float64(m.TotalProcessed) / duration.Seconds()
		fmt.Printf("   Avg Latency:      %.2f ms\n", avgMs)
		fmt.Printf("   Throughput:       %.2f tx/sec\n", tps)
	}

	// Interpretation
	fmt.Printf("\n💡 INTERPRETATION\n")
	if recall >= 0.9 {
		fmt.Println("   ✅ Excellent recall - catching most fraud")
	} else if recall >= 0.7 {
		fmt.Println("   ⚠️  Good recall - but missing some fraud")
	} else if recall >= 0.5 {
		fmt.Println("   ⚠️  Moderate recall - significant fraud being missed")
	} else {
		fmt.Println("   ❌ Poor recall - most fraud is being missed!")
	}

	if precision >= 0.5 {
		fmt.Println("   ✅ Good precision - alerts are meaningful")
	} else if precision >= 0.2 {
		fmt.Println("   ⚠️  Low precision - many false alarms")
	} else {
		fmt.Println("   ❌ Very low precision - mostly false alarms")
	}

	fmt.Println()
}
