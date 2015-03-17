// Unused. This was the previous main function that started a webhook listening server.
// Websockets and Slack's RTM API are used now. See websockets.go

package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"strings"
)

import "github.com/JamesDunne/go-util/base"
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

	text := formValues["text"]
	text = strings.Trim(text, " \t\n")

	// Log incoming text:
	log.Printf(
		"#%s <%s (%s)>: %s\n",
		formValues["channel_name"],
		user_name,
		formValues["user_id"],
		text,
	)

	// Strip "bitsbot" prefix off text:
	// NOTE(jsd): "@bitsbot" does not trigger with outgoing webhooks via trigger words.
	if strings.HasPrefix(text, "bitsbot") {
		text = strings.TrimLeft(text[len("bitsbot"):], " :\t\n")
	}

	// Convert formValues into SlackInMessage:
	slackMessage := &SlackInMessage{
		UserID:      formValues["user_id"],
		UserName:    user_name,
		ChannelID:   formValues["channel_id"],
		ChannelName: formValues["channel_name"],
		Text:        text,
		Timestamp:   formValues["timestamp"],
	}

	// Handle the chat message:
	jsonResponse, werr := botHandleMessage(slackMessage)
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

func mainWebhookServer() error {
	var fl_listen_uri string
	flag.StringVar(&fl_listen_uri, "l", "tcp://0.0.0.0:8080", "Listen address")
	flag.Parse()

	listen_addr, err := base.ParseListenable(fl_listen_uri)
	base.PanicIf(err)

	if err := parseEnv([]string{"BIT_AUTH", "SLACK_TOKEN"}); err != nil {
		return err
	}

	// Start the webhook server:
	_, err = base.ServeMain(listen_addr, func(l net.Listener) error {
		return http.Serve(l, web.ReportErrors(web.Log(web.DefaultErrorLog, web.ErrorHandlerFunc(processRequest))))
	})

	// Return any error:
	return err
}
