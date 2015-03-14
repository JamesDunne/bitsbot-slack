package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
)

import "github.com/JamesDunne/go-util/base"
import "github.com/JamesDunne/go-util/web"

var env map[string]string

func parseEnv(requiredEnv []string) bool {
	// Get required environment variables:
	missing := make([]string, 0, len(requiredEnv))
	env = make(map[string]string)
	for _, name := range requiredEnv {
		value := os.Getenv(name)
		if value == "" {
			missing = append(missing, name)
		}
		env[name] = value
	}
	if len(missing) > 0 {
		log.Printf("Missing required environment variables: %v\n", missing)
		return false
	}
	return true
}

func main() {
	var fl_listen_uri string
	flag.StringVar(&fl_listen_uri, "l", "tcp://0.0.0.0:8080", "Listen address")
	flag.Parse()

	listen_addr, err := base.ParseListenable(fl_listen_uri)
	base.PanicIf(err)

	if !parseEnv([]string{"BIT_AUTH", "SLACK_TOKEN"}) {
		return
	}

	// Start the server:
	_, err = base.ServeMain(listen_addr, func(l net.Listener) error {
		return http.Serve(l, web.ReportErrors(web.Log(web.DefaultErrorLog, web.ErrorHandlerFunc(processRequest))))
	})
	if err != nil {
		log.Println(err)
		return
	}
}
