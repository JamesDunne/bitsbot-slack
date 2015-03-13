package main

import (
	"log"
	"net"
	"net/http"
)

import "github.com/JamesDunne/go-util/base"
import "github.com/JamesDunne/go-util/web"

func main() {
	fl_listen_uri := "tcp://0.0.0.0:8080"
	listen_addr, err := base.ParseListenable(fl_listen_uri)
	base.PanicIf(err)

	// Start the server:
	_, err = base.ServeMain(listen_addr, func(l net.Listener) error {
		return http.Serve(l, web.ReportErrors(web.Log(web.DefaultErrorLog, web.ErrorHandlerFunc(processRequest))))
	})
	if err != nil {
		log.Println(err)
		return
	}
}
