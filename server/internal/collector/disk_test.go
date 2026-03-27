package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const testProcMounts = `/dev/sda1 / ext4 rw,relatime 0 0
proc /proc proc rw,nosuid 0 0
sysfs /sys sysfs rw,nosuid 0 0
tmpfs /tmp tmpfs rw,nosuid 0 0
/dev/sda2 /home ext4 rw,relatime 0 0
`

const testDiskstats = `   8       0 sda 12345 0 98765 1234 56789 0 87654 5678 0 3456 6912
   8       1 sda1 10000 0 80000 1000 50000 0 70000 5000 0 3000 6000
   8       2 sda2 2345 0 18765 234 6789 0 17654 678 0 456 912
`

const testDiskstats2 = `   8       0 sda 12445 0 99765 1334 56889 0 88654 5778 0 3556 7012
   8       1 sda1 10100 0 80800 1100 50100 0 70800 5100 0 3100 6100
   8       2 sda2 2345 0 18765 234 6789 0 17654 678 0 456 912
`

func TestDiskCollector_Name(t *testing.T) {
	d := NewDiskCollector()
	if d.Name() != "disk" {
		t.Errorf("expected name 'disk', got %q", d.Name())
	}
}

func TestDiskCollector_CollectUsage(t *testing.T) {
	dir := t.TempDir()
	mountsPath := filepath.Join(dir, "mounts")
	diskstatsPath := filepath.Join(dir, "diskstats")

	// Write a simple mounts file pointing to actual filesystems we can statfs
	realMounts := "rootfs / ext4 rw 0 0\n"
	os.WriteFile(mountsPath, []byte(realMounts), 0644)
	os.WriteFile(diskstatsPath, []byte(testDiskstats), 0644)

	d := NewDiskCollector()
	d.procMounts = mountsPath
	d.procDiskstats = diskstatsPath

	metrics, err := d.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected at least one metric")
	}
}

func TestDiskCollector_IODelta(t *testing.T) {
	dir := t.TempDir()
	mountsPath := filepath.Join(dir, "mounts")
	diskstatsPath := filepath.Join(dir, "diskstats")

	os.WriteFile(mountsPath, []byte(""), 0644)
	os.WriteFile(diskstatsPath, []byte(testDiskstats), 0644)

	d := NewDiskCollector()
	d.procMounts = mountsPath
	d.procDiskstats = diskstatsPath

	// First collect (no IO delta)
	_, err := d.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Update diskstats
	os.WriteFile(diskstatsPath, []byte(testDiskstats2), 0644)

	// Second collect (should have IO metrics)
	metrics, err := d.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Look for disk_io category
	foundIO := false
	for _, m := range metrics {
		if m.Category == "disk_io" {
			foundIO = true
			if m.Values["read_bytes_per_sec"] < 0 {
				t.Error("read speed should be non-negative")
			}
		}
	}
	if !foundIO {
		t.Error("expected disk_io metrics on second collect")
	}
}

func TestIsPartition(t *testing.T) {
	tests := []struct {
		name   string
		expect bool
	}{
		{"sda", false},
		{"sda1", true},
		{"vda", false},
		{"vda1", true},
		{"nvme0n1", false},
		{"nvme0n1p1", true},
		{"dm-0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPartition(tt.name)
			if got != tt.expect {
				t.Errorf("isPartition(%q) = %v, want %v", tt.name, got, tt.expect)
			}
		})
	}
}

func TestIsRealFS(t *testing.T) {
	if !isRealFS("ext4") {
		t.Error("ext4 should be a real filesystem")
	}
	if isRealFS("proc") {
		t.Error("proc should not be a real filesystem")
	}
	if isRealFS("sysfs") {
		t.Error("sysfs should not be a real filesystem")
	}
}
