package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NetworkCollector collects network traffic metrics from /proc/net/dev.
type NetworkCollector struct {
	interval   time.Duration
	procNetDev string
	mu         sync.Mutex
	prev       map[string]*netStats
	prevTime   time.Time
}

type netStats struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
}

func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		interval:   5 * time.Second,
		procNetDev: "/proc/net/dev",
		prev:       make(map[string]*netStats),
	}
}

func (n *NetworkCollector) Name() string            { return "network" }
func (n *NetworkCollector) Interval() time.Duration { return n.interval }

func (n *NetworkCollector) Collect(ctx context.Context) ([]*Metrics, error) {
	now := time.Now()

	current, err := n.readNetDev()
	if err != nil {
		return nil, fmt.Errorf("network collect: %w", err)
	}

	// Aggregate all interfaces
	var totalRx, totalTx, totalRxPkt, totalTxPkt uint64
	for _, s := range current {
		totalRx += s.RxBytes
		totalTx += s.TxBytes
		totalRxPkt += s.RxPackets
		totalTxPkt += s.TxPackets
	}

	n.mu.Lock()
	prev := n.prev
	prevTime := n.prevTime
	n.prev = current
	n.prevTime = now
	n.mu.Unlock()

	// First collect or no previous data
	if len(prev) == 0 || prevTime.IsZero() {
		return []*Metrics{{
			Category:  "network",
			Timestamp: now,
			Values: map[string]float64{
				"rx_bytes":      float64(totalRx),
				"tx_bytes":      float64(totalTx),
				"rx_bytes_rate": 0,
				"tx_bytes_rate": 0,
				"rx_packets":    float64(totalRxPkt),
				"tx_packets":    float64(totalTxPkt),
				"connections":   float64(countTCPConnections()),
			},
		}}, nil
	}

	elapsed := now.Sub(prevTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	var prevTotalRx, prevTotalTx uint64
	for _, s := range prev {
		prevTotalRx += s.RxBytes
		prevTotalTx += s.TxBytes
	}

	rxRate := float64(totalRx-prevTotalRx) / elapsed
	txRate := float64(totalTx-prevTotalTx) / elapsed

	results := []*Metrics{{
		Category:  "network",
		Timestamp: now,
		Values: map[string]float64{
			"rx_bytes":      float64(totalRx),
			"tx_bytes":      float64(totalTx),
			"rx_bytes_rate": rxRate,
			"tx_bytes_rate": txRate,
			"rx_packets":    float64(totalRxPkt),
			"tx_packets":    float64(totalTxPkt),
			"connections":   float64(countTCPConnections()),
		},
	}}

	// Per-interface metrics
	for iface, cur := range current {
		p, ok := prev[iface]
		if !ok {
			continue
		}
		ifRxRate := float64(cur.RxBytes-p.RxBytes) / elapsed
		ifTxRate := float64(cur.TxBytes-p.TxBytes) / elapsed

		results = append(results, &Metrics{
			Category:  "network",
			Timestamp: now,
			Values: map[string]float64{
				"rx_bytes_rate": ifRxRate,
				"tx_bytes_rate": ifTxRate,
				"rx_bytes":      float64(cur.RxBytes),
				"tx_bytes":      float64(cur.TxBytes),
			},
			Labels: map[string]string{
				"interface": iface,
			},
		})
	}

	return results, nil
}

func (n *NetworkCollector) readNetDev() (map[string]*netStats, error) {
	f, err := os.Open(n.procNetDev)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stats := make(map[string]*netStats)
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			continue // skip header lines
		}

		line := scanner.Text()
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}

		iface := strings.TrimSpace(line[:colonIdx])
		if iface == "lo" {
			continue // skip loopback
		}

		fields := strings.Fields(line[colonIdx+1:])
		if len(fields) < 10 {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		rxPackets, _ := strconv.ParseUint(fields[1], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
		txPackets, _ := strconv.ParseUint(fields[9], 10, 64)

		stats[iface] = &netStats{
			RxBytes:   rxBytes,
			TxBytes:   txBytes,
			RxPackets: rxPackets,
			TxPackets: txPackets,
		}
	}

	return stats, scanner.Err()
}

// countTCPConnections counts established TCP connections from /proc/net/tcp.
func countTCPConnections() int {
	f, err := os.Open("/proc/net/tcp")
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		// st field (connection state): 01 = ESTABLISHED
		if fields[3] == "01" {
			count++
		}
	}
	return count
}
