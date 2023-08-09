package socketIO

import (
	"net/http"
	"sync"

	engineIO "github.com/ghuvrons/go.engine.io"
	"github.com/google/uuid"
)

type Options struct {
	PingInterval int
	PingTimeout  int
}

type Server struct {
	engineio   *engineIO.Server
	Sockets    Sockets // [TODO] make it private
	socketsMtx *sync.Mutex

	handlers struct {
		connection func(*Socket)
	}

	Rooms         map[string]*Room // key: roomName
	authenticator func(interface{}) bool
}

func NewServer(opt Options) *Server {
	eioOptions := engineIO.Options{
		PingInterval: opt.PingInterval,
		PingTimeout:  opt.PingTimeout,

		BasePath: "/socket.io/",
	}

	server := &Server{
		engineio:   engineIO.NewServer(eioOptions),
		Sockets:    Sockets{},
		socketsMtx: &sync.Mutex{},
		Rooms:      map[string]*Room{},
	}

	server.engineio.OnConnection(func(conn *engineIO.Socket) {
		newClient(server, conn)
	})

	return server
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.engineio.ServeHTTP(w, r)
}

func (server *Server) Authenticator(f func(interface{}) bool) {
	server.authenticator = f
}

func (server *Server) onNewSocket(socket *Socket) {
	server.socketsMtx.Lock()
	server.Sockets[socket.id] = socket
	server.socketsMtx.Unlock()

	if server.handlers.connection != nil {
		go server.handlers.connection(socket)
	}
}

func (server *Server) onClosedSocket(socket *Socket) {
	server.socketsMtx.Lock()
	delete(server.Sockets, socket.id)
	server.socketsMtx.Unlock()
}

func (server *Server) OnConnection(f func(*Socket)) {
	server.handlers.connection = f
}

// Room methods
func (server *Server) CreateRoom(roomName string) (room *Room) {
	room = &Room{
		Name:    roomName,
		sockets: map[uuid.UUID]*Socket{},
	}

	if server.Rooms == nil {
		server.Rooms = map[string]*Room{}
	}

	server.Rooms[roomName] = room
	return
}

func (server *Server) DeleteRoom(roomName string) {
	delete(server.Rooms, roomName)
}
