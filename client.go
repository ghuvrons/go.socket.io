package socketIO

import (
	"bytes"
	"reflect"

	engineIO "github.com/ghuvrons/go.engine.io"
)

type client struct {
	server    *Server
	conn      *engineIO.Socket
	sockets   map[string]*Socket // key: namespace
	tmpSocket *Socket
}

func newClient(server *Server, conn *engineIO.Socket) *client {
	c := &client{
		server:  server,
		conn:    conn,
		sockets: map[string]*Socket{},
	}

	conn.OnMessage(c.handleMessage)
	conn.OnClosed(c.handleClose)

	return c
}

func (c *client) handleMessage(message interface{}) {
	var msgBytes []byte
	var msgString string
	isBinary := false

	switch data := message.(type) {
	case string:
		msgString = data
	case []byte:
		isBinary = true
		msgBytes = data
	default:
		return
	}

	if isBinary {
		if c.tmpSocket == nil {
			return
		}

		if packet := c.tmpSocket.tmpPacket; packet != nil && packet.numOfBuffer > 0 {
			buf := bytes.NewBuffer(msgBytes)
			sioPacketSetBuffer(packet.data, buf)
			packet.numOfBuffer--

			// buffering complete
			if packet.numOfBuffer == 0 {
				c.tmpSocket.onPacket(packet)
				c.tmpSocket.tmpPacket = nil
				c.tmpSocket = nil
			}
		}
		return
	}

	buf := bytes.NewBuffer([]byte(msgString))
	packet := decodeToPacket(buf)
	if packet == nil {
		return
	}

	if packet.packetType == __SIO_PACKET_CONNECT {
		socket := newSocket(c.server, packet.namespace)
		socket.eioSocket = c.conn

		if c.server.authenticator != nil {
			if !c.server.authenticator(packet.data) {
				errConnData := map[string]interface{}{
					"message": "Not authorized",
					"data": map[string]interface{}{
						"code":  "E001",
						"label": "Invalid credentials",
					},
				}
				socket.send(newPacket(__SIO_PACKET_CONNECT_ERROR, errConnData))
				return
			}
		}
		socket.connect(packet)

		c.sockets[packet.namespace] = socket
		c.server.onNewSocket(socket)
		return
	}

	socket, isFound := c.sockets[packet.namespace]
	if !isFound || socket == nil {
		return
	}

	switch packet.packetType {
	case __SIO_PACKET_EVENT, __SIO_PACKET_BINARY_EVENT:
		if packet.packetType == __SIO_PACKET_BINARY_EVENT {
			// TODO check this
			// c.conn.IsReadingPayload = true
			socket.tmpPacket = packet
			c.tmpSocket = socket
		} else {
			socket.onPacket(packet)
		}
		return

	case __SIO_PACKET_DISCONNECT:
		socket.onClosing()
		socket.onClose()
	}
}

func (c *client) handleClose() {
	for _, socket := range c.sockets {
		socket.Close(0)
	}
}

// search {"_placeholder":true,"num":n} and replace with buffer
func sioPacketSetBuffer(v interface{}, buf *bytes.Buffer) (isFound bool, isReplaced bool, err error) {
	rv := reflect.ValueOf(v)

	if rk := rv.Kind(); rk == reflect.Ptr || rk == reflect.Interface {
		rv = rv.Elem()
	}

	if !rv.IsValid() || ((rv.Kind() != reflect.Map && rv.Kind() != reflect.Slice) && !rv.CanSet()) {
		return false, false, nil
	}

	switch rk := rv.Kind(); rk {
	case reflect.Ptr:
		if !rv.IsNil() {
			if isFound, isReplaced, err = sioPacketSetBuffer(rv.Interface(), buf); isFound || err != nil {
				return
			}
		}

	case reflect.Map:
		flag := 2
		keys := rv.MapKeys()

		if len(keys) == 2 {
			for _, key := range keys {
				strKey := key.String()
				if strKey != "_placeholder" && strKey != "num" {
					break
				}

				rvv := rv.MapIndex(key)
				if rkv := rvv.Kind(); rkv == reflect.Interface {
					rvv = rvv.Elem()
				}

				if rkv := rvv.Kind(); (strKey == "_placeholder" && rkv == reflect.Bool) || (strKey == "num" && rkv == reflect.Float64) {
					flag--
				}
			}

			if flag == 0 { // buffer req found
				return true, false, nil
			}
		}

		for _, key := range keys {
			rvv := rv.MapIndex(key)
			isFound, isReplaced, err = sioPacketSetBuffer(rvv.Interface(), buf)
			if isFound && !isReplaced {
				rv.SetMapIndex(key, reflect.ValueOf(buf))
				isReplaced = true
			}
			if isFound || err != nil {
				break
			}
		}

	case reflect.Array, reflect.Slice:
		for j := 0; j < rv.Len(); j++ {
			rvv := rv.Index(j)
			if rkv := rvv.Kind(); rkv == reflect.Interface {
				rvv = rvv.Elem()
			}
			isFound, isReplaced, err = sioPacketSetBuffer(rvv.Interface(), buf)
			if isFound && !isReplaced {
				if rvv = rv.Index(j); !isReplaced && rvv.CanSet() {
					rvv.Set(reflect.ValueOf(buf))
					isReplaced = true
				}
			}
			if isFound || err != nil {
				break
			}
		}
	}
	return
}

// search buffer and replace with {"_placeholder":true,"num":n}
func sioPacketGetBuffer(buffers *([]*bytes.Buffer), v interface{}) bool {
	rv := reflect.ValueOf(v)

	if rk := rv.Kind(); rk == reflect.Ptr {
		rv = rv.Elem()
	}

	if !rv.IsValid() || ((rv.Kind() != reflect.Map && rv.Kind() != reflect.Slice) && !rv.CanSet()) {
		return false
	}

	switch rk := rv.Kind(); rk {
	case reflect.Ptr:
		return sioPacketGetBuffer(buffers, rv.Interface())

	case reflect.Struct:
		if rv.Type() == typeOfBuffer {
			return true
		}
		for i := 0; i < rv.NumField(); i++ {
			rvField := rv.Field(i)
			if rvField.CanSet() && rvField.CanAddr() {
				sioPacketGetBuffer(buffers, rvField.Addr().Interface())
			}
		}

	case reflect.Map:
		keys := rv.MapKeys()

		for _, key := range keys {
			rvv := rv.MapIndex(key)
			if isFound := sioPacketGetBuffer(buffers, rvv.Interface()); isFound {
				bufPtr, isOk := rvv.Interface().(*bytes.Buffer)
				if isOk {
					*buffers = append(*buffers, bufPtr)
					rv.SetMapIndex(key, reflect.ValueOf(socketIOBufferIndex))
				}
			}
		}

	case reflect.Array, reflect.Slice:
		for j := 0; j < rv.Len(); j++ {
			rvv := rv.Index(j)
			if rkv := rvv.Kind(); rkv == reflect.Interface {
				if rvv.IsNil() {
					continue
				}
				rvv = rvv.Elem()
			}
			if isFound := sioPacketGetBuffer(buffers, rvv.Interface()); isFound {
				bufPtr, isOk := rvv.Interface().(*bytes.Buffer)
				if isOk {
					*buffers = append(*buffers, bufPtr)
					rv.Index(j).Set(reflect.ValueOf(socketIOBufferIndex))
				}
			}
		}

	case reflect.Interface:
		if rv.IsNil() {
			return false
		}
		rvv := rv.Elem()
		if isFound := sioPacketGetBuffer(buffers, rvv.Interface()); isFound {
			bufPtr, isOk := rvv.Interface().(*bytes.Buffer)
			if isOk && rv.CanSet() && rv.CanAddr() {
				*buffers = append(*buffers, bufPtr)
				rv.Set(reflect.ValueOf(socketIOBufferIndex))
			}
		}
	}
	return false
}
