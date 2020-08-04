package zhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type SSENotificationCenter struct {
	subscribers   map[chan []byte]struct{}
	subscribersMu *sync.Mutex
}

func SetupSSE(pathWithPort string) *SSENotificationCenter {
	nc := &SSENotificationCenter{
		subscribers:   map[chan []byte]struct{}{},
		subscribersMu: &sync.Mutex{},
	}
	http.HandleFunc("/sse", handleSSE(nc))

	return nc
}

func handleSSE(nc *SSENotificationCenter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Subscribe
		c := make(chan []byte)

		nc.subscribersMu.Lock()
		nc.subscribers[c] = struct{}{}
		nc.subscribersMu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

	Looping:
		for {
			select {
			case <-r.Context().Done():
				nc.subscribersMu.Lock()
				delete(nc.subscribers, c)
				nc.subscribersMu.Unlock()
				break Looping

			default:
				b := <-c
				fmt.Fprintf(w, "data: %s\n\n", b)
				w.(http.Flusher).Flush()
			}
		}
	}
}

func (nc *SSENotificationCenter) NotifyWithJson(v interface{}) error {
	nc.subscribersMu.Lock()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	defer nc.subscribersMu.Unlock()
	for c := range nc.subscribers {
		select {
		case c <- data:
		default:
		}
	}

	return nil
}
