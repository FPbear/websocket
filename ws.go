package ws

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
)

type sendMsg struct {
	msg []byte
}

type recvMsg struct {
	msg []byte
}

type Message interface {
	GetCmd() uint16
	GetMsg() []byte
	GetData(pm proto.Message) proto.Message
}

func (m *recvMsg) GetCmd() uint16 {
	return binary.LittleEndian.Uint16(m.msg[:2])
}

func (m *recvMsg) GetMsg() []byte {
	return m.msg[6:]
}

func (m *recvMsg) GetData(pm proto.Message) proto.Message {
	err := proto.Unmarshal(m.msg[6:], pm)
	if err != nil {
		return nil
	}
	return pm
}

type WebSocketTask interface {
	Handle(handle func(data Message) (uint16, proto.Message))
}

type WsTask struct {
	// closed Determines whether the connection is closed
	closed int32
	// verified Client active flag
	// The client can be kicked out by judging that the field has timed out
	verified bool
	// stopChan Stop sending coroutine when the connection is closed
	//stopChan  chan bool
	sendMutex sync.Mutex
	Conn      *websocket.Conn
	msgChan   chan *sendMsg
	recvChan  chan *recvMsg
	ctx       context.Context
	cancel    context.CancelFunc
	//Derived   IWebSocketTask
}

func NewWsTask(w http.ResponseWriter, r *http.Request, conf *ServerConfig, ) (WebSocketTask, error) {
	var svr *websocket.Upgrader
	if conf == nil {
		svr = defaultServer()
	} else {
		svr = newServer(conf)
	}
	conn, err := svr.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ws := &WsTask{
		closed:   -1,
		verified: false,
		Conn:     conn,
		msgChan:  make(chan *sendMsg, 10),
		recvChan: make(chan *recvMsg, 100),
		ctx:      ctx,
		cancel:   cancel,
	}
	ws.start()
	return ws, nil
}

// Start Task process on
func (ws *WsTask) start() {
	if !atomic.CompareAndSwapInt32(&ws.closed, -1, 0) {
		return
	}
	go ws.recvLoop()
	go ws.sendLoop()
}

// Verify Heartbeat packet update
func (ws *WsTask) Verify() {
	ws.verified = true
}

// IsVerified Client liveness detection
func (ws *WsTask) IsVerified() bool {
	return ws.verified
}

// IsClosed Determine if the connection is closed
func (ws *WsTask) isClosed() bool {
	return atomic.LoadInt32(&ws.closed) != 0
}

// Send server wants to send the data interface to the client
func (ws *WsTask) send(buffer []byte) bool {
	if ws.isClosed() {
		return false
	}
	ws.msgChan <- &sendMsg{msg: buffer}
	return true
}

func (ws *WsTask) Handle(handle func(data Message) (uint16, proto.Message)) {
	for {
		select {
		case <-ws.ctx.Done():
			return
		case msg, ok := <-ws.recvChan:
			if !ok || msg == nil {
				ws.canal()
				fmt.Println("Handle Exit")
				return
			}
			s, err := encodeMsg(handle(msg))
			if err != nil {
				continue
			}
			ws.send(s)
		}
	}
}

// Close Processing connection closed
func (ws *WsTask) Close() {
	if !atomic.CompareAndSwapInt32(&ws.closed, 0, 1) {
		return
	}
	_ = ws.Conn.Close()
	close(ws.recvChan)
	close(ws.msgChan)
}

// recvLoop receive and process client messages
func (ws *WsTask) recvLoop() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	defer ws.Close()
	var (
		messageType int
		message     []byte
		err         error
	)
	for {
		messageType, message, err = ws.Conn.ReadMessage()
		if err != nil {
			ws.cancel()
			if !websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway) {
				//log.WithFields(log.Fields{"function": "WsTask.recvLoop"}).Info(err)
				return
			}
			fmt.Println(err)
			return
		}
		ws.Verify()
		if messageType != websocket.BinaryMessage {
			_ = ws.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			ws.cancel()
			fmt.Println("msg format error.")
			return
		}
		length := len(message)
		if length < 8 {
			fmt.Println("msg is short.")
			continue
		}
		if int(binary.LittleEndian.Uint16(message[:2])) != length {
			fmt.Println("msg length error.")
			continue
		}
		ws.recvChan <- &recvMsg{msg: message[2:]}
		message = message[:0]
		messageType = 0
	}
}

// sendLoop Listen to the server and send messages to the client.
// Listens for server and client heartbeats.
// Actively close the sending interface.
func (ws *WsTask) sendLoop() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	defer ws.Close()
	var (
		timeout = time.NewTicker(time.Second * 60)
		err     error
	)
	for {
		select {
		case <-ws.ctx.Done():
			return
		case message, ok := <-ws.msgChan:
			if !ok {
				return
			}
			err = ws.Conn.WriteMessage(websocket.BinaryMessage, message.msg)
			if err != nil {
				fmt.Println(err)
				return
			}
		case <-timeout.C:
			if !ws.IsVerified() {
				ws.cancel()
				fmt.Println("is not heart package")
				return
			}
		}
	}
}

func defaultServer() *websocket.Upgrader {
	return &websocket.Upgrader{
		HandshakeTimeout: 30 * time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: false,
	}
}

type ServerConfig struct {
	HandshakeTimeout  int64
	ReadBufferSize    int
	WriteBufferSize   int
	CheckOrigin       bool
	EnableCompression bool
}

func newServer(conf *ServerConfig) *websocket.Upgrader {
	return &websocket.Upgrader{
		HandshakeTimeout: time.Duration(conf.HandshakeTimeout) * time.Second,
		ReadBufferSize:   conf.ReadBufferSize,
		WriteBufferSize:  conf.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return conf.CheckOrigin
		},
		EnableCompression: conf.EnableCompression,
	}
}
