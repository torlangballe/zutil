package zrpc

import (
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

type MessageID struct {
	ID   string
	Type string
}

type MessageBoardRequest struct {
	SessionID int32
	ID        string
	Type      string
	Message   string
	Reply     string
	Answering bool
	Answered  bool
}

var messageBoardLock sync.Mutex
var messageBoardRequests = map[int32]*MessageBoardRequest{}

func KeepAskingForMessage(client *Client, everySecs float64, messageType, messageID string, got func(m *MessageBoardRequest) bool) {
	m := MessageID{ID: messageID, Type: messageType}
	ztimer.RepeatIn(everySecs, func() bool {
		var newMessage MessageBoardRequest
		err := client.CallRemote(Calls.GetMessage, &m, &newMessage)
		if err != nil && newMessage.SessionID != 0 {
			if got(&newMessage) {
				err = client.CallRemote(Calls.AnswerMessage, &newMessage, nil)
				if err != nil {
					zlog.Error(err, "answer call")
				}
			}
		}
		return true
	})
}

// GetMessage returns a sent  message as MessageBoardRequest if it
// has want.ID and want.Type if not empty. Returns a message with 0 SessionID if none available
func (c *RPCCalls) GetMessage(req *http.Request, want *MessageID, mbreq *MessageBoardRequest) error {
	for i := 0; i < 80; i++ {
		messageBoardLock.Lock()
		for _, m := range messageBoardRequests {
			if want.ID == m.ID && (want.Type == "" || want.Type == m.Type) && !m.Answering {
				messageBoardLock.Unlock()
				m.Answering = true
				*mbreq = *m
				return nil
			}
		}
		messageBoardLock.Unlock()
		time.Sleep(time.Millisecond * 100)
	}
	*mbreq = MessageBoardRequest{}
	return nil
}

func (c *RPCCalls) SendMessage(req *http.Request, mreq *MessageBoardRequest, reply *string) error {
	session := rand.Int31()
	mreq.SessionID = session
	messageBoardLock.Lock()
	messageBoardRequests[session] = mreq
	messageBoardLock.Unlock()
	for i := 0; i < 80; i++ {
		time.Sleep(time.Millisecond * 100)
		if mreq.Answered {
			*reply = mreq.Reply
			messageBoardLock.Lock()
			delete(messageBoardRequests, session)
			messageBoardLock.Unlock()
			return nil
		}
	}
	return zlog.Error(nil, "never got reply in time")
}

func (c *RPCCalls) AnswerMessage(req *http.Request, mreq *MessageBoardRequest, reply *Any) error {
	messageBoardLock.Lock()
	found := messageBoardRequests[mreq.SessionID]
	messageBoardLock.Unlock()
	if found == nil {
		return zlog.Error(nil, "no session in message board requests", mreq.SessionID)
	}
	*found = *mreq
	found.Answered = true
	return nil
}
