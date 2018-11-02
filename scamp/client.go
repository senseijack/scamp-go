package scamp

import (
	"sync"
)

// type ClientChan chan *Client

// Client represents a scamp client
type Client struct {
	conn           *Connection
	service        *Service
	requests       chan *Message
	openRepliesMut sync.Mutex
	openReplies    map[int]chan *Message
	isClosed       bool
	closedMut      sync.Mutex
	sendMut        sync.Mutex
	nextRequestID  int
	spIdent        string
	closeOnce      sync.Once
	closeReqChan   sync.Once
}

// Dial calls DialConnection to establish a secure (tls) connection,
// and uses that connection to create a client
func Dial(connspec string) (client *Client, err error) {
	conn, err := DialConnection(connspec)
	if err != nil {
		return
	}
	client = NewClient(conn)
	return
}

// NewClient takes a scamp connection and creates a new scamp client
func NewClient(conn *Connection) (client *Client) {
	client = &Client{
		conn:        conn,
		requests:    make(chan *Message), //TODO: investigate using buffered channel here
		openReplies: make(map[int]chan *Message),
	}
	conn.SetClient(client)

	go client.splitReqsAndReps()

	return
}

// SetService assigns a *Service to client.serv
func (client *Client) SetService(s *Service) {
	client.service = s
}

// Send TODO: would be nice to have different code path for scamp responses
// so that we don't need to rely on garbage collection of channels
// when we're replying and don't expect or need a response
func (client *Client) Send(msg *Message) (responseChan chan *Message, err error) {
	client.sendMut.Lock()
	defer client.sendMut.Unlock()

	client.nextRequestID++
	msg.RequestID = client.nextRequestID
	err = client.conn.Send(msg)
	if err != nil {
		// Trace.Printf("SCAMP send error: %s", err)
		return
	}

	if msg.MessageType == MessageTypeRequest {
		// Trace.Printf("sending request so waiting for reply")
		responseChan = make(chan *Message)
		client.openRepliesMut.Lock()
		client.openReplies[msg.RequestID] = responseChan
		client.openRepliesMut.Unlock()
	} else {
		// TODO: refactor this
		// Trace.Printf("sending reply so done with this message")
	}
	return
}

// Close ensures that client.close() is only called once
func (client *Client) Close() {
	client.closeOnce.Do(func() {
		client.close()
	})
}

func (client *Client) close() {
	if len(client.spIdent) > 0 {
		defaultCacheMut.Lock()
		sp := defaultCache.Retrieve(client.spIdent)
		defaultCacheMut.Unlock()
		if sp != nil {
			sp.client = nil
		}
	}

	client.closedMut.Lock()
	defer client.closedMut.Unlock()

	if client.isClosed {
		return
	}
	client.closeReqChan.Do(func() {
		close(client.requests)
	})
	client.closeConnection(client.conn)

	// Notify wrapper service that we're dead
	if client.service != nil {
		client.service.RemoveClient(client)
	}
	client.isClosed = true
}

// closeConnection calls client.conn.Close() and sets the client.conn to nil
func (client *Client) closeConnection(conn *Connection) {
	if !client.conn.isClosed {
		client.conn.close()
	}
	client.conn = nil
}

//func (client *Client) splitReqsAndReps(grNum, clientID int) (err error) {
func (client *Client) splitReqsAndReps() (err error) {
	var replyChan chan *Message

	// TODO: need to make sure this finishes BEFORE we close client or else we have a race condition
	// and potential panic()
ForLoop:
	for {
		select {
		// case <-client.service.quit:
		// break ForLoop //TODO: we should wait until all messages are received
		case message, ok := <-client.conn.msgs: //race
			if !ok {
				// Trace.Printf("client.conn.msgs... CLOSED!")
				break ForLoop
			}
			if message == nil {
				continue ForLoop
			}

			// Trace.Printf("Splitting incoming message to reqs and reps")

			if message.MessageType == MessageTypeRequest {
				// TODO: bad things happen if there are outstanding messages
				// and the client closes
				client.requests <- message
			} else if message.MessageType == MessageTypeReply {
				client.openRepliesMut.Lock()
				// TODO: this is kind of clunky maybe refactor?
				replyChan = client.openReplies[message.RequestID]
				if replyChan == nil {
					// Error.Printf("got an unexpected reply for requestId: %d. Skipping.", message.RequestID)
					client.openRepliesMut.Unlock()
					continue ForLoop
				}

				delete(client.openReplies, message.RequestID)
				client.openRepliesMut.Unlock()

				replyChan <- message
			} else {
				Warning.Printf("Could not handle msg, it's neither req or reply. Skipping.")
				continue ForLoop
			}
		}
	}
	// client.closeReqChan.Do(func() {
	// 	close(client.requests)
	// })
	client.openRepliesMut.Lock()
	for _, openReplyChan := range client.openReplies {
		close(openReplyChan) //TODO: Once
	}

	// client.openRepliesMut.Unlock()
	close(replyChan)
	client.Close()
	return
}

// Incoming returns a client's MessageChan
func (client *Client) Incoming() chan *Message {
	return client.requests
}
