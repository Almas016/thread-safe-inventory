# REVIEW.md

## Race Condition 1: GetStock reads shared state without synchronization

**Code:**

```go
func (s *InventoryService) GetStock(productID string) int {
	product := s.products[productID]
    if product == nil {
        return 0
    }
    return product.Stock
}
```

**What happens:**

`GetStock` reads shared state without any lock.

It reads from `s.products` and then reads `product.Stock`.
If another goroutine updates the same product at the same time,
this creates a read/write data race.

**Production scenario:**

Goroutine 1 handles `GET /stock/p1` and calls:

`GetStock("p1")`

At the same time Goroutine 2 calls:

`Reserve("p1", 1)`

One goroutine reads `product.Stock` while another writes it.

**Fix approach:**

Use `sync.RWMutex` stored inside the service.

`GetStock` should use `RLock/RUnlock`.
Write methods should use `Lock/Unlock`.

--------------------------------------------------

## Race Condition 2: Reserve performs check and update without atomic synchronization

**Code:**
```go
func (s *InventoryService) Reserve(productID string, quantity int) error {
    product := s.products[productID]

    if product == nil {
        return ErrProductNotFound
    }

    if product.Stock < quantity {
        return ErrInsufficientStock
    }

    product.Stock -= quantity
    return nil
}
```

**What happens:**

The stock check and stock update are separate unsynchronized operations.

Multiple goroutines may read the same stock value,
all pass validation, then all subtract stock.

This causes overselling.

**Production scenario:**

Stock = 5

Goroutine 1:
`Reserve("p1", 5)`

Goroutine 2:
`Reserve("p1", 5)`

Both read stock = 5
Both succeed
Both subtract 5

Two successful reservations for only five items.

**Fix approach:**

Use one write lock for the full operation:

- read product
- validate stock
- subtract stock

Do not unlock between check and update.

--------------------------------------------------

## Race Condition 3: ReserveMultiple has race between validation and update phases

**Code:**
```go
func (s *InventoryService) ReserveMultiple(items []ReserveItem) error {
    for _, item := range items {
        product := s.products[item.ProductID]
        if product.Stock < item.Quantity {
            return ErrInsufficientStock
        }
    }

    for _, item := range items {
        s.products[item.ProductID].Stock -= item.Quantity
    }

    return nil
}
```

**What happens:**

`ReserveMultiple` works in two phases:

1. validate all products
2. update all products

Between those phases another goroutine may change stock.

Reads and writes happen without synchronization.

This breaks all-or-nothing behavior.

**Production scenario:**

A = 10
B = 10

Goroutine 1:

`ReserveMultiple(A:8, B:8)`

Validation passes.

Before update, Goroutine 2 runs:

`Reserve("B", 5)`

Now B no longer has enough stock,
but Goroutine 1 still subtracts 8.

**Fix approach:**

Use one write lock around the entire `ReserveMultiple` method.

Inside one critical section:

- verify products exist
- verify all stock is enough
- update all products

Only update after all checks pass.

--------------------------------------------------

## Race Condition 4: SafeReserve creates a new mutex per call

**Code:**
```go
func (s *InventoryService) SafeReserve(productID string, quantity int) error {
    var mu sync.Mutex
    mu.Lock()
    defer mu.Unlock()

    product := s.products[productID]

    if product.Stock < quantity {
        return ErrInsufficientStock
    }

    product.Stock -= quantity
    return nil
}

```

**What happens:**

The mutex is local inside the function.

Each call creates a new mutex.

So goroutines lock different mutexes,
not the same shared mutex.

That means there is no real synchronization.

**Production scenario:**

Goroutine 1 creates `mu1` and locks it.
Goroutine 2 creates `mu2` and locks it.

`mu1` and `mu2` are different locks.

Both goroutines continue concurrently
and race on product.Stock.

**Fix approach:**

Move mutex into the service struct:

```go
type SafeInventoryService struct {
    mu sync.RWMutex
    products map[string]*Product
}
```

All goroutines must use the same mutex instance.