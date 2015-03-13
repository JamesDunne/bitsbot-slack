package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

//import "github.com/JamesDunne/go-util/base"
import "github.com/JamesDunne/go-util/web"

func processRequest(rsp http.ResponseWriter, req *http.Request) *web.Error {
	if err := req.ParseForm(); err != nil {
		log.Printf("Could not parse form: %s", err)
		return web.AsError(err, http.StatusBadRequest)
	}

	// TODO: validate 'token'
	//req.PostForm.Get("token")

	log.Println("Got form post:")
	for name, values := range req.PostForm {
		for _, value := range values {
			log.Printf("  %s=%s\n", name, value)
		}
	}

	// Prevent infinite echos:
	if req.PostForm.Get("user_name") == "slackbot" {
		rsp.WriteHeader(http.StatusOK)
		return nil
	}

	// Strip "bitsbot" prefix off text:
	text := strings.Trim(req.PostForm.Get("text"), " \t\n")
	if strings.HasPrefix(text, "bitsbot") {
		text = strings.Trim(text[len("bitsbot"):], " \t\n")
	}

	// Echo back text:
	rsp.Header().Set("Content-Type", "application/json")
	rsp.WriteHeader(http.StatusOK)
	json.NewEncoder(rsp).Encode(struct {
		Text string `json:"text"`
	}{
		Text: text,
	})
	return nil
}
