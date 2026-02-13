package zwebsocket

import (
	"fmt"
)

type Exchanger interface {
	Exchange(msg []byte) ([]byte, error)
}

type Exchangers map[string]Exchanger

var ExchangerNotFound = fmt.Errorf("exchanger not found")

func (e *Exchangers) ExchangeWithID(id string, msg []byte) ([]byte, error) {
	exchanger, got := (*e)[id]
	// zlog.Warn("Exchanger ExchangeWithID id:", got, id, "msg:", string(msg))
	if !got {
		return nil, ExchangerNotFound
	}
	return exchanger.Exchange(msg)
}
