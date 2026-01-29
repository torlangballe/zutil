package zwebsocket

import "fmt"

type Exchanger interface {
	Exchange(msg []byte) ([]byte, error)
}

type IDExchanger interface {
	Exchange(id string, msg []byte) ([]byte, error)
}

type Exchangers map[string]Exchanger

var ExchangerNotFound = fmt.Errorf("exchanger not found")

func (e *Exchangers) Exchange(id string, msg []byte) ([]byte, error) {
	exchanger, got := (*e)[id]
	if !got {
		return nil, ExchangerNotFound
	}
	return exchanger.Exchange(msg)
}

/*
type MessageHandler interface {
	HandleMessage(id string, msg []byte, err error) []byte
}

type ExchangerPool struct {
	exchangers map[string]Exchanger
	Handler    MessageHandler
}

func NewExchangerPool() *ExchangerPool {
	return &ExchangerPool{
		exchangers: make(map[string]Exchanger),
	}
}

func (p *ExchangerPool) Exchange(id string, msg []byte) ([]byte, error) {
	e, got := p.exchangers[id]
	if !got {
		return nil, fmt.Errorf("no exchanger for id:%s", id)
	}
	return e.Exchange(msg)
}

func (p *ExchangerPool) SetExchanger(id string, exchanger Exchanger) {
	p.exchangers[id] = exchanger
}

func (p *ExchangerPool) RemoveExchanger(id string) {
	delete(p.exchangers, id)
}
*/
