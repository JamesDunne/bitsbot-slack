package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

import "github.com/JamesDunne/go-util/web"

// Webhook server for Outgoing Webhook integration with Slack.
func processRequest(rsp http.ResponseWriter, req *http.Request) *web.Error {
	if err := req.ParseForm(); err != nil {
		log.Printf("Could not parse form: %s\n", err)
		return web.AsError(err, http.StatusBadRequest)
	}

	formValues := make(map[string]string)
	for key, values := range req.PostForm {
		formValues[key] = strings.Join(values, " ")
	}

	if formValues["token"] != env["SLACK_TOKEN"] {
		// Not meant for us.
		rsp.WriteHeader(http.StatusOK)
		return nil
	}

	// Don't accept messages not intended for us:
	if formValues["trigger_word"] != "bitsbot" {
		rsp.WriteHeader(http.StatusOK)
		return nil
	}

	// Prevent infinite echos:
	user_name := formValues["user_name"]
	if user_name == "slackbot" {
		rsp.WriteHeader(http.StatusOK)
		return nil
	}

	// Log incoming text:
	log.Printf(
		"#%s <%s (%s)>: %s\n",
		formValues["channel_name"],
		user_name,
		formValues["user_id"],
		formValues["text"],
	)

	// Handle the chat message:
	jsonResponse, werr := handleChatMessage(formValues)
	if werr != nil {
		return werr
	}

	// Write JSON response with attached image:
	rsp.Header().Set("Content-Type", "application/json")
	rsp.WriteHeader(http.StatusOK)

	o, err := json.Marshal(jsonResponse)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return nil
	}

	//log.Printf("%s\n", string(o))
	rsp.Write(o)

	return nil
}
