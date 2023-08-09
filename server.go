package socketIO

import (
	"net/http"
	"sync"

	engineIO "github.com/ghuvrons/go.engine.io"
)

type Options struct {
	PingInterval int
	PingTimeout  int
}

type Server struct {
	engineio   *engineIO.Server
	socketsMtx *sync.Mutex

	handlers struct {
		connection func(*Socket)
	}

	// list of sockets
	namespaces map[string]*Namespace // key: namespace
	rooms      map[string]*Room      // key: roomName

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
		socketsMtx: &sync.Mutex{},

		namespaces: map[string]*Namespace{},
		rooms:      map[string]*Room{},
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
	ns := server.getNamespace(socket.namespace)
	ns.sockets[socket.id] = socket
	server.socketsMtx.Unlock()

	if server.handlers.connection != nil {
		go server.handlers.connection(socket)
	}
}

func (server *Server) onClosedSocket(socket *Socket) {
	server.socketsMtx.Lock()
	ns := server.getNamespace(socket.namespace)
	delete(ns.sockets, socket.id)
	server.socketsMtx.Unlock()
}

func (server *Server) OnConnection(f func(*Socket)) {
	server.handlers.connection = f
}

// Room methods
func (server *Server) getRoom(roomName string) *Room {
	room, isFound := server.rooms[roomName]
	if !isFound {
		room = &Room{
			Name:    roomName,
			sockets: Sockets{},
		}

		if server.rooms == nil {
			server.rooms = map[string]*Room{}
		}

		server.rooms[roomName] = room
	}
	return room
}

func (server *Server) deleteRoom(roomName string) {
	delete(server.rooms, roomName)
}

// Namespace methods
func (server *Server) getNamespace(name string) *Namespace {
	ns, isFound := server.namespaces[name]
	if !isFound {
		ns = &Namespace{
			name:    name,
			sockets: Sockets{},
		}

		if server.namespaces == nil {
			server.namespaces = map[string]*Namespace{}
		}

		server.namespaces[name] = ns
	}
	return ns
}
