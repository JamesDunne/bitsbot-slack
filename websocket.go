package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

import "golang.org/x/net/websocket"

var bot_tag string

type BotConnection struct {
	ws        *websocket.Conn
	waitGroup *sync.WaitGroup
	die       chan bool

	user_names    map[string]string
	channel_names map[string]string
}

// Send a ping every 15 seconds to avoid EOFs.
func (bc *BotConnection) pingpong() {
	defer bc.waitGroup.Done()

	ticker := time.Tick(time.Second * 15)

	alive := true
	for alive {
		select {
		case <-ticker:
			ping := struct {
				Type string `json:"type"`
			}{
				Type: "ping",
			}
			//log.Println("pingpong: send ping")
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

type wsInMessage map[string]interface{}

func (bc *BotConnection) resolveUserName(userID string) string {
	// TODO: lock!
	if userName, ok := bc.user_names[userID]; ok {
		return userName
	}

	// Look up the user name:
	rsp, err := slackAPI("users.info", map[string]string{
		"user": userID,
	})
	if err != nil {
		return ""
	}

	// Getting sloppy here...
	userName := rsp["user"].(map[string]interface{})["name"].(string)
	bc.user_names[userID] = userName

	return userName
}

func (bc *BotConnection) resolveChannelName(channelID string) string {
	if channelName, ok := bc.channel_names[channelID]; ok {
		return channelName
	}

	// Look up the channel name:
	rsp, err := slackAPI("channels.info", map[string]string{
		"channel": channelID,
	})
	if err != nil {
		return ""
	}

	// Getting sloppy here...
	channelName := rsp["channel"].(map[string]interface{})["name"].(string)
	bc.channel_names[channelID] = channelName

	return channelName
}

// goroutine to handle incoming messages:
func (bc *BotConnection) handleMessage(wsInMessage *wsInMessage) {
	msg := *wsInMessage

	// Handle messages based on type:
	msgType := msg["type"]
	switch msgType {
	case "message":
		// Ignore messages not sent directly to this bot:
		text := msg["text"].(string)
		if !strings.HasPrefix(text, bot_tag) {
			break
		}

		// Handle chat message:
		slackMessage := &SlackInMessage{
			UserID:    msg["user"].(string),
			ChannelID: msg["channel"].(string),
			Text:      text,
			Timestamp: msg["ts"].(string),
		}

		// Resolve user name and channel name:
		slackMessage.UserName = bc.resolveUserName(slackMessage.UserID)
		slackMessage.ChannelName = bc.resolveChannelName(slackMessage.ChannelID)

		log.Printf("  #%s <%s>: %s\n", slackMessage.ChannelID, slackMessage.UserID, slackMessage.Text)

		// Let the bot do its thing:
		rsp, err := botHandleMessage(slackMessage)
		if err != nil {
			log.Println(err)
			break
		}

		// TODO: Break up response text if 16k JSON limit hit.

		websocket.JSON.Send(bc.ws, rsp)
		break

	// Ignore these kinds:
	case "pong", "user_typing", "presence_change":
		break
	default:
		log.Printf("  type '%s': %+v\n", msgType, msg)
		break
	}
}

// Read incoming messages:
func (bc *BotConnection) readIncomingMessages() {
	defer func() {
		log.Println("incoming: dying")
		bc.die <- true
		bc.waitGroup.Done()
	}()

	for {
		// Receive a message:
		wsInMessage := make(wsInMessage)
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

		// Handle the message:
		go bc.handleMessage(&wsInMessage)
	}
}

func watchdog() {
	for {
		// Connect to slack API:
		log.Println("Connecting to Slack API with rtm.start...")
		rsp, err := slackAPI("rtm.start", nil)
		if err != nil {
			log.Println(err)
			continue
		}

		// Dial websocket:
		wsURL := rsp["url"].(string)
		log.Printf("Dialing Slack websocket '%s'...\n", wsURL)
		ws, err := websocket.Dial(wsURL, "", "http://localhost/")
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println("Connected to Slack websocket.")

		bc := &BotConnection{
			ws:        ws,
			waitGroup: new(sync.WaitGroup),
			die:       make(chan bool),

			user_names:    make(map[string]string),
			channel_names: make(map[string]string),
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
	if err := parseEnv([]string{ /*"BIT_AUTH", */ "SLACK_TOKEN", "BOT_USERID"}); err != nil {
		return err
	}

	// BOT_USERID should be something like "U03wxyz", not the bot's user name.
	bot_tag = fmt.Sprintf("<@%s>", env["BOT_USERID"])

	// Start the websocket connector watchdog:
	watchdog()

	return nil
}
