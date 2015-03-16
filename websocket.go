package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
)

import "golang.org/x/net/websocket"

func watchdog() {
	for {
		// Connect to slack API:
		log.Println("Connecting to Slack API with rtm.start...")
		rsp, err := http.Get("https://slack.com/api/rtm.start?token=" + url.QueryEscape(env["SLACK_TOKEN"]))
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("Connected to Slack API; reading response...")

		// Read JSON response:
		// https://api.slack.com/methods/rtm.start
		wsURLResponse := &struct {
			Ok  bool   `json:"ok"`
			URL string `json:"url"`
		}{}
		err = json.NewDecoder(rsp.Body).Decode(wsURLResponse)
		if err != nil {
			log.Println(err)
			continue
		}
		if !wsURLResponse.Ok {
			log.Println(wsURLResponse)
			continue
		}

		// Dial websocket:
		log.Printf("Dialing websocket '%s'\n", wsURLResponse.URL)
		ws, err := websocket.Dial(wsURLResponse.URL, "", "")
		if err != nil {
			log.Println(err)
			continue
		}

		// Handle incoming messages:
		log.Println("Connected to Slack websocket.")
		for {
			log.Println("  Awaiting incoming message")

			// Receive a message:
			wsInMessage := make(map[string]interface{})
			err = websocket.JSON.Receive(ws, &wsInMessage)

			// Handle connection errors:
			if err != nil {
				log.Println(err)
				break
			}

			// Handle semantic errors:
			if errJson, ok := wsInMessage["error"]; ok {
				log.Println(errJson)
				continue
			}

			// Handle messages based on type:
			msgType := wsInMessage["type"]
			switch msgType {
			default:
				log.Printf("  %s: %s\n", msgType, wsInMessage)
				break
			}
		}
	}
}

func mainWebsocketClient() error {
	//flag.Parse()

	// Get sensitive information from environment variables:
	if err := parseEnv([]string{ /*"BIT_AUTH", */ "SLACK_TOKEN"}); err != nil {
		return err
	}

	// Start the websocket connector watchdog:
	watchdog()

	return nil
}
