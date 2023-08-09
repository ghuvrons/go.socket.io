package main

import (
	"fmt"
	"net/http"

	socketIO "github.com/ghuvrons/go.socket.io"
)

type socketIOServer struct {
	sioServer *socketIO.Server
}

func main() {
	http.Handle("/socket.io/", socketIOInit())
	http.Handle("/", http.FileServer(http.Dir("./client")))

	err := http.ListenAndServe(":3333", nil)
	fmt.Println(err)
}

func socketIOInit() http.Handler {
	io := socketIO.NewServer(socketIO.Options{
		PingInterval: 5000,
		PingTimeout:  10000,
	})
	// sioHandler.Authenticator(func(data interface{}) bool {
	// 	fmt.Println("auth data", data)
	// 	return true
	// })

	io.OnConnection(func(socket *socketIO.Socket) {
		fmt.Println("new socket", socket)

		socket.On("message", func(data ...interface{}) {
			fmt.Println("new message", data)

			// cbdata := map[string]interface{}{
			// 	"data": bytes.NewBuffer([]byte{1, 2, 3}),
			// }
			// socket.Emit("test_bin", cbdata)
		})

		socket.On("send-buffer", func(data ...interface{}) {
			fmt.Println("new buffer message", data)
		})
		// cbdata := map[string]interface{}{
		// 	"data": bytes.NewBuffer([]byte{1, 2, 3}),
		// }
		// socket.Emit("test_bin", cbdata)
	})

	h := socketIOServer{}
	h.sioServer = io

	return h
}

func (svr socketIOServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	svr.sioServer.ServeHTTP(w, req)
}
