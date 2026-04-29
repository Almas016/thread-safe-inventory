# Answers

## Q1: Why is the mutex inside `SafeReserve` useless?

The mutex is declared inside the function, so every call creates a new separate mutex instance.

That means different goroutines do not lock the same mutex.

Example:

- Goroutine 1 calls `SafeReserve` and creates `mu1`
- Goroutine 2 calls `SafeReserve` and creates `mu2`
- `mu1` and `mu2` are different locks

So both goroutines can enter the critical section at the same time and still race on `product.Stock`.

A mutex only works when all goroutines synchronize through the same shared mutex instance. The mutex must be stored in the service struct, not created locally inside the method.

---

## Q2: What can happen with per-product locks in `ReserveMultiple`?

A deadlock can happen.

Example:

- Goroutine 1 locks Product A and waits for Product B
- Goroutine 2 locks Product B and waits for Product A

Now both goroutines are waiting for each other forever.

To prevent this, locks must always be acquired in a deterministic order. For example, sort product IDs and always lock products by ascending product ID.

Another simpler approach is to use one service-level mutex for the whole `ReserveMultiple` operation. This avoids deadlocks and guarantees all-or-nothing behavior.

---

## Q3: Why is releasing the lock early worse?

This code splits one atomic operation into separate locked sections:

1. lock and read product pointer
2. unlock
3. check stock without lock
4. lock again and subtract stock

The bug is that stock can change between the check and the update.

Example:

Product stock is `1`.

- Goroutine 1 reads product and unlocks
- Goroutine 2 reads product and unlocks
- Both see stock as enough
- Both subtract `1`

Final stock can become `-1`, and the system accepts two reservations for only one available item.

This is worse because the code looks safe due to locks, but the actual critical operation is not atomic. The check and update must be protected by the same lock without releasing it in between.

---

## Q4: If `go test -race` shows no warnings, does it mean the code is race-free?

No.

`go test -race` only detects races that actually happen during that specific test run.

If the tests do not cover a problematic execution path, or if the goroutine timing does not trigger the race, the race detector may show no warnings even though the code is still unsafe.

Also, the race detector finds data races, but it does not prove high-level correctness. For example, it may not detect logical concurrency bugs such as:

- broken atomicity
- wrong lock ordering
- deadlocks that did not occur in the test
- all-or-nothing violations

So `-race` is very useful, but it is not a formal proof that the program is completely race-free.