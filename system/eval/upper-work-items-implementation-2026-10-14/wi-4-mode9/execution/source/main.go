package main

// Controlled RPC timing fixture. It accepts the existing launch/config shape,
// not blockchain consensus. No production validity is asserted by this fixture.
import (
	"context"
	"encoding/json"
	"fmt"
	ksobserve "log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const fixed = false
const activation = "0x0000000000000000000000000000000000000001"

func main() {
	for _, a := range os.Args[1:] {
		if a == "init" {
			return
		}
	}
	config := ""
	for i, a := range os.Args[1:] {
		if a == "--config" && i+2 < len(os.Args) {
			config = os.Args[i+2]
		}
	}
	raw, e := os.ReadFile(config)
	if e != nil {
		panic(e)
	}
	match := regexp.MustCompile(`(?m)^\s*HTTPPort\s*=\s*([0-9]+)`).FindSubmatch(raw)
	if len(match) != 2 {
		panic("owned HTTP port unavailable")
	}
	port, e := strconv.Atoi(string(match[1]))
	if e != nil {
		panic(e)
	}
	var once sync.Once
	var ready atomic.Bool
	committed := make(chan struct{})
	server := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			ID     any
			Method string
			Params []json.RawMessage
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			http.Error(w, "bad request", 400)
			return
		}
		var result any
		switch q.Method {
		case "eth_chainId":
			result = "0x205b"
		case "eth_blockNumber":
			result = fmt.Sprintf("0x%x", time.Now().UnixMilli()/500)
		case "net_peerCount":
			result = "0x3"
		case "eth_syncing":
			result = false
		case "eth_getBlockByNumber":
			result = map[string]any{"hash": "0x" + strings.Repeat("0", 64)}
		case "eth_getBalance":
			var address string
			if len(q.Params) > 0 {
				_ = json.Unmarshal(q.Params[0], &address)
			}
			if address == activation {
				once.Do(func() { go func() { time.Sleep(300 * time.Millisecond); ready.Store(true); close(committed) }() })
				result = "0x0"
			} else {
				if fixed {
					select {
					case <-committed:
					case <-r.Context().Done():
						return
					}
				}
				observedReady := ready.Load()
				ksobserve.Printf(`KS_TIMING_V1 {"schema_version":1,"probe_id":"read-probe","event":"read-before-commit","site":"main/read","symptom":%t,"values":{"ready":%t}}`, !observedReady, observedReady)
				result = "0x0"
				if observedReady {
					result = "0x1"
				}
			}
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "error": map[string]any{"code": -32601, "message": "unsupported controlled fixture method"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result})
	})}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	select {
	case e := <-done:
		if e != http.ErrServerClosed {
			panic(e)
		}
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}
