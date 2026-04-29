package thread_safe_inventory

import (
	"errors"
	"sync"
	"testing"
)

func TestReserve_ConcurrentOversell(t *testing.T) {
	service := NewSafeInventoryService(map[string]*Product{
		"p1": {
			ID:    "p1",
			Name:  "Product 1",
			Stock: 100,
		},
	})

	var wg sync.WaitGroup
	var mu sync.Mutex

	successCount := 0
	failCount := 0

	for i := 0; i < 200; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := service.Reserve("p1", 1)

			mu.Lock()
			defer mu.Unlock()

			if err == nil {
				successCount++
				return
			}

			if errors.Is(err, ErrInsufficientStock) {
				failCount++
				return
			}

			t.Errorf("unexpected error: %v", err)
		}()
	}

	wg.Wait()

	if successCount != 100 {
		t.Fatalf("expected 100 successful reservations, got %d", successCount)
	}

	if failCount != 100 {
		t.Fatalf("expected 100 failed reservations, got %d", failCount)
	}

	stock := service.GetStock("p1")
	if stock != 0 {
		t.Fatalf("expected final stock to be 0, got %d", stock)
	}
}

func TestReserveMultiple_Atomicity(t *testing.T) {
	service := NewSafeInventoryService(map[string]*Product{
		"A": {
			ID:    "A",
			Name:  "Product A",
			Stock: 10,
		},
		"B": {
			ID:    "B",
			Name:  "Product B",
			Stock: 5,
		},
	})

	err := service.ReserveMultiple([]ReserveItem{
		{
			ProductID: "A",
			Quantity:  8,
		},
		{
			ProductID: "B",
			Quantity:  8,
		},
	})

	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}

	stockA := service.GetStock("A")
	if stockA != 10 {
		t.Fatalf("expected Product A stock to remain 10, got %d", stockA)
	}

	stockB := service.GetStock("B")
	if stockB != 5 {
		t.Fatalf("expected Product B stock to remain 5, got %d", stockB)
	}
}
