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
		log.Printf("Could not parse form: %s\n", err)
		return web.AsError(err, http.StatusBadRequest)
	}

	// TODO: validate 'token'
	//req.PostForm.Get("token")

	// Don't accept messages not intended for us:
	if req.PostForm.Get("trigger_word") != "bitsbot" {
		rsp.WriteHeader(http.StatusOK)
		return nil
	}

	// Prevent infinite echos:
	if req.PostForm.Get("user_name") == "slackbot" {
		rsp.WriteHeader(http.StatusOK)
		return nil
	}

	// Log incoming text:
	log.Printf(
		"#%s <%s (%s)>: %s\n",
		req.PostForm.Get("channel_name"),
		req.PostForm.Get("user_name"),
		req.PostForm.Get("user_id"),
		req.PostForm.Get("text"),
	)

	// NOTE(jsd): "@bitsbot" does not trigger with outgoing webhooks via trigger words.
	// Strip "bitsbot" prefix off text:
	text := strings.Trim(req.PostForm.Get("text"), " \t\n")
	if strings.HasPrefix(text, "bitsbot") {
		text = strings.TrimLeft(text[len("bitsbot"):], " :\t\n")
	}

	// Text is HTML encoded otherwise.

	// Debug tool for user "jdunne":
	if strings.HasPrefix(text, "json=") && req.PostForm.Get("user_id") == "U03PV154T" {
		// Remove angle brackets around URLs:
		text = strings.Replace(text, "<", "", -1)
		text = strings.Replace(text, ">", "", -1)

		// Unmarshal JSON:
		o := make(map[string]interface{})
		err := json.Unmarshal([]byte(text[len("json="):]), &o)
		if err != nil {
			log.Printf("ERROR: %s\n", err)
			goto otherwise
		}

		// Echo incoming JSON data as our response:
		rsp.Header().Set("Content-Type", "application/json")
		rsp.WriteHeader(http.StatusOK)
		json.NewEncoder(rsp).Encode(o)
		return nil
	}

otherwise:
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
