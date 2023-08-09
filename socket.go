package socketIO

import (
	engineIO "github.com/ghuvrons/go.engine.io"
	"github.com/google/uuid"
)

type Socket struct {
	id           uuid.UUID
	server       *Server
	eioSocket    *engineIO.Socket
	namespace    string
	eventEmitter *eventEmitter
	tmpPacket    *packet

	// when Socket get events with ack. that ACK will be saved to this
	ackIdHandling int

	handlers struct {
		disconnecting func(reason int)
		disconnect    func(reason int)
	}

	// rooms that connected by this socket
	rooms map[string]*Room // key: roomName
}

type Sockets map[uuid.UUID]*Socket

// newSocket create new Socket
func newSocket(server *Server, namespace string) *Socket {
	return &Socket{
		server:       server,
		id:           uuid.New(),
		namespace:    namespace,
		eventEmitter: newEventEmitter(),
		rooms:        map[string]*Room{},
	}
}

// Handle socket's connect request
func (socket *Socket) connect(conpacket *packet) error {
	// do authenticating ...

	// if success
	socket.send(newPacket(__SIO_PACKET_CONNECT, map[string]interface{}{"sid": socket.id.String()}))

	return nil
}

func (socket *Socket) send(p *packet) {
	p.namespace = socket.namespace
	encodedPacket, buffers := p.encode()

	if len(buffers) == 0 {
		socket.eioSocket.Send(encodedPacket)

	} else {
		// binary message
		if err := socket.eioSocket.Send(encodedPacket); err != nil {
			return
		}

		for _, buf := range buffers {
			if err := socket.eioSocket.Send(buf.Bytes()); err != nil {
				return
			}
		}
	}
}

func (socket *Socket) Emit(arg ...interface{}) {
	socket.send(newPacket(__SIO_PACKET_EVENT, arg...))
}

func (socket *Socket) On(event string, f func(...interface{})) {
	socket.eventEmitter.On(event, f)
}

func (socket *Socket) onPacket(p *packet) {
	args, isOk := p.data.([]interface{})
	defer func() {
		socket.ackIdHandling = 0
	}()

	if isOk && len(args) > 0 {
		socket.eventEmitter.Emit("message", args...)

		switch args[0].(type) {
		case string:
			event := args[0].(string)
			if p.ackId >= 0 {
				socket.ackIdHandling = p.ackId
				socket.eventEmitter.Emit(event, append(args[1:], socket.callbackAck)...)
			} else {
				socket.eventEmitter.Emit(event, args[1:]...)
			}
		}
	}
}

func (socket *Socket) callbackAck(arg ...interface{}) {
	socket.send(newPacket(__SIO_PACKET_ACK, arg...).withAck(socket.ackIdHandling))
}

func (socket *Socket) OnDisconnecting(f func(reason int)) {
	socket.handlers.disconnecting = f
}

func (socket *Socket) OnDisconnect(f func(reason int)) {
	socket.handlers.disconnect = f
}

func (socket *Socket) Close(reason int) {
	socket.onClosing()
	for _, room := range socket.rooms {
		room.leave(socket)
	}
	socket.send(newPacket(__SIO_PACKET_DISCONNECT))
	socket.onClose()
}

func (socket *Socket) onClosing() {
	if socket.handlers.disconnecting != nil {
		socket.handlers.disconnecting(0)
	}
}

func (socket *Socket) onClose() {
	socket.server.onClosedSocket(socket)

	if socket.handlers.disconnect != nil {
		socket.handlers.disconnect(0)
	}
}

func (socket *Socket) SocketJoin(roomName string) {
	room, isFound := socket.server.Rooms[roomName]
	if !isFound {
		room = socket.server.CreateRoom(roomName)
	}
	room.join(socket)
}

func (socket *Socket) SocketLeave(roomName string) {
	room, isFound := socket.server.Rooms[roomName]
	if !isFound {
		return
	}
	room.leave(socket)
}

func (sockets Sockets) Emit(arg ...interface{}) {
	packet := newPacket(__SIO_PACKET_EVENT, arg...)
	for _, socket := range sockets {
		socket.send(packet)
	}
}

func (sockets Sockets) SocketJoin(roomName string) {
	var server *Server = nil

	if len(sockets) == 0 {
		return
	}

	for _, socket := range sockets {
		server = socket.server
		break
	}

	room, isFound := server.Rooms[roomName]
	if !isFound {
		room = server.CreateRoom(roomName)
	}

	for _, socket := range sockets {
		room.join(socket)
	}
}

func (sockets Sockets) SocketLeave(roomName string) {
	var server *Server = nil

	if len(sockets) == 0 {
		return
	}

	for _, socket := range sockets {
		server = socket.server
		break
	}
	room, isFound := server.Rooms[roomName]
	if !isFound {
		return
	}

	for _, socket := range sockets {
		room.leave(socket)
	}

	if len(room.sockets) == 0 {
		server.DeleteRoom(roomName)
	}
}
