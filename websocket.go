package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

import "golang.org/x/net/websocket"

type BotConnection struct {
	ws        *websocket.Conn
	die       chan bool
	waitGroup *sync.WaitGroup
}

// Send a ping every 10 seconds to avoid EOFs.
func (bc *BotConnection) pingpong() {
	defer bc.waitGroup.Done()

	ticker := time.Tick(time.Second * 10)

	alive := true
	for alive {
		select {
		case <-ticker:
			ping := struct {
				Type string `json:"type"`
			}{
				Type: "ping",
			}
			log.Println("pingpong: send ping")
			err := websocket.JSON.Send(bc.ws, &ping)
			if err != nil {
				log.Println(err)
			}
			break
		case <-bc.die:
			log.Println("pingpong: dying")
			alive = false
			break
		}
	}
}

// Read incoming messages:
func (bc *BotConnection) readIncomingMessages() {
	defer func() {
		log.Println("incoming: dying")
		bc.die <- true
		bc.waitGroup.Done()
	}()

	// Handle incoming messages:
	for {
		log.Println("  Awaiting incoming message")

		// Receive a message:
		wsInMessage := make(map[string]interface{})
		err := websocket.JSON.Receive(bc.ws, &wsInMessage)

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
		ws, err := websocket.Dial(wsURLResponse.URL, "", "http://localhost/")
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("Connected to Slack websocket.")

		bc := &BotConnection{
			ws:        ws,
			waitGroup: new(sync.WaitGroup),
		}

		bc.waitGroup.Add(2)
		go bc.pingpong()
		go bc.readIncomingMessages()

		// wait for goroutines:
		bc.waitGroup.Wait()
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
