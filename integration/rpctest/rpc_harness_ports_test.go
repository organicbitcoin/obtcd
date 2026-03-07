package rpctest

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestNextAvailablePortFromFileSequential(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "ports.lock")
	portFile := filepath.Join(tempDir, "ports.state")

	first := nextAvailablePortFromFile(lockFile, portFile)
	second := nextAvailablePortFromFile(lockFile, portFile)
	third := nextAvailablePortFromFile(lockFile, portFile)

	if first <= int(defaultNodePort) {
		t.Fatalf("expected first allocated port to be above default start, got %d", first)
	}
	if second <= first {
		t.Fatalf("expected second allocated port to be greater than first: %d <= %d", second, first)
	}
	if third <= second {
		t.Fatalf("expected third allocated port to be greater than second: %d <= %d", third, second)
	}
}

func TestNextAvailablePortFromFileConcurrent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "ports.lock")
	portFile := filepath.Join(tempDir, "ports.state")

	const workers = 8

	results := make(chan int, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- nextAvailablePortFromFile(lockFile, portFile)
		}()
	}

	wg.Wait()
	close(results)

	seen := make(map[int]struct{}, workers)
	for port := range results {
		if _, exists := seen[port]; exists {
			t.Fatalf("duplicate port allocated: %d", port)
		}
		seen[port] = struct{}{}
	}

	if len(seen) != workers {
		t.Fatalf("expected %d unique ports, got %d", workers, len(seen))
	}
}

func TestNextAvailablePortFromFileWrapsCursor(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	lockFile := filepath.Join(tempDir, "ports.lock")
	portFile := filepath.Join(tempDir, "ports.state")

	writeLastAllocatedPort(portFile, 65535)

	port := nextAvailablePortFromFile(lockFile, portFile)
	if port <= int(defaultNodePort) || port >= 65535 {
		t.Fatalf("expected wrapped port within valid range, got %d", port)
	}
}
