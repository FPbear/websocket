
package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"ws"
	"net/http"
)

func testServer(w http.ResponseWriter, r *http.Request) {
	svr, err := ws.NewWsTask(w, r,nil)
	if err != nil {
		fmt.Println(err)
		_, _ = w.Write([]byte("error"))
		return
	}
	defer func() {
		// todo close your business
	}()
	svr.Handle(func(data ws.Message) (uint16, proto.Message) {
		switch data.GetCmd() {
		case 1:
			// todo handle business
			// param,ok:=data.GetData(&pb.OneMsg).(*pb.OneMsg)
			return 1, nil
		default:
			return 0, nil
		}
	})
}

func main() {
	mux := http.DefaultServeMux
	mux.HandleFunc("/testing", testServer)
	if err := http.ListenAndServe(":8000", mux); err != nil {
		fmt.Println(err)
	}
	return
}
