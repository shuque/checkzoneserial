# checkzoneserial - Design

## What the program does

checkzoneserial queries the SOA (Start of Authority) record for a DNS zone at all of its authoritative nameservers and reports the serial number and response time observed from each. This is useful for monitoring DNS zone propagation -- verifying that all servers are serving the same version of a zone.

If a master server is specified (`-m`), the program queries the master first, then compares each server's serial against it, reporting the delta. An allowed drift threshold (`-d`) can be set so that small differences don't trigger a failure.

The program exits with status codes suitable for scripting and monitoring:
- **0**: all serials match (or are within allowed drift)
- **1**: serial mismatch or drift exceeded
- **2**: server issues (timeout, bad response, no serials obtained)
- **3**: master server failure
- **4**: program invocation error

Output can be plain text or JSON (`-j`).

## Program structure

The code is organized into four source files:

- **`main.go`** -- entry point, program orchestration, and all core logic
- **`options.go`** -- command-line flag parsing and the `Options`/`QueryOptions` types
- **`query.go`** -- low-level DNS query construction and transport (UDP, TCP, UDP-with-TCP-fallback)
- **`sorting.go`** -- DNS canonical name ordering and IP version sorting for output

## Execution flow

1. **Flag parsing** (`doFlags`): parses CLI flags into an `Options` struct, validates inputs, and returns the target zone name.

2. **Resolver setup** (`GetResolver`): reads the system's `resolv.conf` (or an alternate file) to obtain recursive resolver addresses used for NS and address lookups.

3. **Nameserver discovery**: the zone's NS records are looked up via the recursive resolver (`getNSnames`). Additional servers can be specified with `-a`, and advertised NS lookups can be skipped with `-n`. Each nameserver hostname is resolved to its A and/or AAAA addresses (`getIPAddresses`), producing a list of `Request` structs (name + IP pairs).

4. **Master query** (optional): if `-m` is specified, the master is queried first, synchronously. Its serial is stored for delta computation. A master failure exits immediately with status 3.

5. **Concurrent SOA queries**: all authoritative server addresses are queried in parallel (see below).

6. **Result collection and output**: responses are collected, optionally sorted by canonical domain name and IP version, and printed. Serial numbers are compared to determine the exit status.

## Concurrent query design

The parallel SOA querying uses three concurrency primitives held in the `Runner` struct:

- **`wg sync.WaitGroup`** -- tracks the number of in-flight goroutines
- **`tokens chan struct{}`** -- a buffered channel of size 20 acting as a counting semaphore to limit concurrency
- **`results chan *Response`** -- an unbuffered channel through which goroutines deliver their results to the main goroutine

The dispatch works as follows:

```
go func() {
    for _, x := range requests {
        rn.wg.Add(1)           // register a pending goroutine
        rn.tokens <- struct{}{}  // acquire a concurrency token (blocks if 20 are in flight)
        go rn.getSerialAsync(zone, x.nsip, x.nsname, opts)
    }
    rn.wg.Wait()    // wait for all goroutines to finish
    close(rn.results)  // signal the collector that no more results are coming
}()
```

This entire dispatch loop runs in its own goroutine so that the main goroutine can simultaneously consume results from the `results` channel:

```
for r := range rn.results {
    // collect and process each response as it arrives
}
```

Each worker goroutine (`getSerialAsync`) does the following:
1. Calls `getSerial` to send a SOA query and parse the response.
2. Releases its concurrency token (`<-rn.tokens`) after the query completes.
3. Constructs a `Response` struct with the serial, response time, NSID (if requested), error state, and delta from the master (if applicable).
4. Sends the `Response` on `rn.results`.

The token is released after the query but before sending on the results channel. This means the concurrency limit (20) governs the number of simultaneous DNS queries in flight, not the number of goroutines that exist. A goroutine that has finished its query but is blocked waiting to send its result does not hold a token.

When all goroutines complete, `wg.Wait()` returns, the dispatch goroutine closes `rn.results`, and the `range` loop in the main goroutine exits.

## Mutable state

All per-execution mutable state is held in a `Runner` struct, created fresh for each invocation via `NewRunner()`. This includes the synchronization primitives (`wg`, `tokens`, `results`), the accumulated serial list, the response map, and the output structure. This design allows `run()` to be tested in isolation without global state leaking between test cases.

## Serial number comparison

SOA serial numbers use RFC 1982 serial number arithmetic, where the 32-bit number space is treated as circular. The `serialDistance` function computes the unsigned shortest-path distance between two serials, and `maxSerialDrift` finds the maximum pairwise distance across all observed serials. The `serialDelta` function computes a signed difference for per-response display (positive = slave is behind master, negative = slave is ahead).

## DNS transport

Queries are sent via UDP by default, with automatic fallback to TCP if the response is truncated. The `-c` flag forces TCP for all queries. UDP queries are retried up to a configurable number of times on timeout; TCP queries are not retried, as TCP provides reliable delivery. EDNS0 is used with a configurable buffer size (default 1400), and NSID can be requested with `-nsid`.
