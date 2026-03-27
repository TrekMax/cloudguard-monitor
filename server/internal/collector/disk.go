package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DiskCollector collects disk usage and IO metrics.
type DiskCollector struct {
	interval      time.Duration
	procDiskstats string
	procMounts    string
	mu            sync.Mutex
	prevIO        map[string]*diskIOStats
	prevTime      time.Time
}

type diskIOStats struct {
	ReadBytes  uint64
	WriteBytes uint64
	ReadOps    uint64
	WriteOps   uint64
}

func NewDiskCollector() *DiskCollector {
	return &DiskCollector{
		interval:      30 * time.Second,
		procDiskstats: "/proc/diskstats",
		procMounts:    "/proc/mounts",
		prevIO:        make(map[string]*diskIOStats),
	}
}

func (d *DiskCollector) Name() string           { return "disk" }
func (d *DiskCollector) Interval() time.Duration { return d.interval }

func (d *DiskCollector) Collect(ctx context.Context) ([]*Metrics, error) {
	now := time.Now()
	var results []*Metrics

	// Collect partition usage via statfs
	mounts, err := d.readMounts()
	if err != nil {
		return nil, fmt.Errorf("disk collect mounts: %w", err)
	}

	for _, mount := range mounts {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount.mountPoint, &stat); err != nil {
			continue
		}

		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bfree * uint64(stat.Bsize)
		availBytes := stat.Bavail * uint64(stat.Bsize)
		usedBytes := totalBytes - freeBytes

		var usagePercent float64
		if totalBytes > 0 {
			usagePercent = 100.0 * float64(usedBytes) / float64(totalBytes)
		}

		results = append(results, &Metrics{
			Category:  "disk",
			Timestamp: now,
			Values: map[string]float64{
				"total":         float64(totalBytes),
				"used":          float64(usedBytes),
				"free":          float64(availBytes),
				"usage_percent": usagePercent,
			},
			Labels: map[string]string{
				"device":     mount.device,
				"mountpoint": mount.mountPoint,
				"fstype":     mount.fsType,
			},
		})
	}

	// Collect IO stats from /proc/diskstats
	ioStats, err := d.readDiskstats()
	if err == nil {
		d.mu.Lock()
		prev := d.prevIO
		prevTime := d.prevTime
		d.prevIO = ioStats
		d.prevTime = now
		d.mu.Unlock()

		if len(prev) > 0 && !prevTime.IsZero() {
			elapsed := now.Sub(prevTime).Seconds()
			if elapsed > 0 {
				for dev, cur := range ioStats {
					p, ok := prev[dev]
					if !ok {
						continue
					}
					readSpeed := float64(cur.ReadBytes-p.ReadBytes) / elapsed
					writeSpeed := float64(cur.WriteBytes-p.WriteBytes) / elapsed
					readIOPS := float64(cur.ReadOps-p.ReadOps) / elapsed
					writeIOPS := float64(cur.WriteOps-p.WriteOps) / elapsed

					results = append(results, &Metrics{
						Category:  "disk_io",
						Timestamp: now,
						Values: map[string]float64{
							"read_bytes_per_sec":  readSpeed,
							"write_bytes_per_sec": writeSpeed,
							"read_iops":           readIOPS,
							"write_iops":          writeIOPS,
						},
						Labels: map[string]string{
							"device": dev,
						},
					})
				}
			}
		}
	}

	if len(results) == 0 {
		return []*Metrics{{
			Category:  "disk",
			Timestamp: now,
			Values:    map[string]float64{},
		}}, nil
	}

	return results, nil
}

type mountInfo struct {
	device     string
	mountPoint string
	fsType     string
}

func (d *DiskCollector) readMounts() ([]mountInfo, error) {
	f, err := os.Open(d.procMounts)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var mounts []mountInfo
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]

		// Only include real filesystems
		if !isRealFS(fsType) {
			continue
		}
		// Skip duplicates
		if seen[mountPoint] {
			continue
		}
		seen[mountPoint] = true

		mounts = append(mounts, mountInfo{
			device:     device,
			mountPoint: mountPoint,
			fsType:     fsType,
		})
	}

	return mounts, scanner.Err()
}

func isRealFS(fsType string) bool {
	switch fsType {
	case "ext2", "ext3", "ext4", "xfs", "btrfs", "zfs", "ntfs", "vfat", "fat32", "fuseblk", "tmpfs":
		return true
	}
	return false
}

func (d *DiskCollector) readDiskstats() (map[string]*diskIOStats, error) {
	f, err := os.Open(d.procDiskstats)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stats := make(map[string]*diskIOStats)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		devName := fields[2]
		// Skip partitions, only keep whole disks (e.g., sda, vda, nvme0n1)
		if isPartition(devName) {
			continue
		}

		readOps, _ := strconv.ParseUint(fields[3], 10, 64)
		readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
		writeOps, _ := strconv.ParseUint(fields[7], 10, 64)
		writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)

		stats[devName] = &diskIOStats{
			ReadBytes:  readSectors * 512,
			WriteBytes: writeSectors * 512,
			ReadOps:    readOps,
			WriteOps:   writeOps,
		}
	}

	return stats, scanner.Err()
}

func isPartition(name string) bool {
	// sda1, vda1, etc. are partitions
	if len(name) == 0 {
		return false
	}
	last := name[len(name)-1]
	if last >= '0' && last <= '9' {
		// Check if it's like sda1 (not nvme0n1)
		if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "vd") || strings.HasPrefix(name, "hd") {
			return true
		}
		// nvme0n1p1 is a partition, nvme0n1 is not
		if strings.Contains(name, "p") && strings.HasPrefix(name, "nvme") {
			parts := strings.SplitN(name, "p", 2)
			if len(parts) == 2 {
				_, err := strconv.Atoi(parts[1])
				return err == nil
			}
		}
	}
	return false
}
