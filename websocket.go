package main

import (
	"encoding/json"
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

	message_id uint32
}

func (bc *BotConnection) SendReceive(msg interface{}) (rsp map[string]interface{}, err error) {
	// Send:
	err = websocket.JSON.Send(bc.ws, msg)
	if err != nil {
		return nil, err
	}

	// Receive response:
	rsp = make(map[string]interface{})
	err = websocket.JSON.Receive(bc.ws, &rsp)
	if err != nil {
		return nil, err
	}

	return rsp, nil
}

// Send a ping every 15 seconds to avoid EOFs.
func (bc *BotConnection) pingpong() {
	defer func() {
		log.Println("  pingpong: dying")
		bc.waitGroup.Done()
	}()

	// Send first ping to avoid early EOF:
	ping := struct {
		Type string `json:"type"`
	}{
		Type: "ping",
	}
	err := websocket.JSON.Send(bc.ws, &ping)
	if err != nil {
		log.Printf("  pingpong: JSON send error: %s\n", err)
	}

	// Start a timer to tick every 15 seconds:
	ticker := time.Tick(time.Second * 15)

	alive := true
	for alive {
		// Wait on either the timer tick or the `die` channel:
		select {
		case _ = <-ticker:
			//log.Println("  pingpong: ping")
			err = websocket.JSON.Send(bc.ws, &ping)
			if err != nil {
				log.Printf("  pingpong: JSON send error: %s\n", err)
				break
			}
			// NOTE: `readIncomingMessages` will read the "pong" response.
			// Cannot issue a read here because a read is already blocking there.
			break
		case _ = <-bc.die:
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

func divideMessage(msg string, divider int) (msgs []string) {
	if len(msg) <= divider {
		return []string{msg}
	}

	msgs = make([]string, 0, len(msg)/divider+1)

	// t is remaining chars to cut
	t := msg

	for len(t) > divider {
		n := divider
		cut := t[:n]
		// Pull back on cut to find last \n:
		if m := strings.LastIndexAny(cut, "\n"); m > -1 {
			n = m + 1
			cut = t[:n]
		}

		// Add the cut:
		msgs = append(msgs, cut)
		t = t[n:]
	}

	// Add remainder:
	if len(t) > 0 {
		msgs = append(msgs, t)
	}

	return
}

// Handle incoming messages:
func (bc *BotConnection) handleMessage(wsInMessage *wsInMessage) {
	msg := *wsInMessage

	// Handle messages based on type:
	msgType := msg["type"]
	switch msgType {
	case "message":
		subtype := msg["subtype"]
		inmsg := &SlackInMessage{}

		text := ""
		//log.Printf("message: %+v\n", msg)

		if subtype != nil {
			// Handle edited messages:
			if subtype == "message_changed" {
				edited_msg := msg["message"].(map[string]interface{})

				inmsg.UserID = edited_msg["user"].(string)
				inmsg.Timestamp = edited_msg["ts"].(string)

				text = edited_msg["text"].(string)
			} else if subtype == "bot_message" {
				// Skip bot messages.
				break
			}
		} else {
			// Regular message:
			inmsg.UserID = msg["user"].(string)
			inmsg.Timestamp = msg["ts"].(string)

			text = msg["text"].(string)
		}

		// Ignore messages not sent directly to this bot:
		if !strings.HasPrefix(text, bot_tag) {
			break
		}

		// Chop off tag at left:
		text = strings.TrimLeft(text[len(bot_tag):], " :\t\n")
		inmsg.Text = text
		inmsg.ChannelID = msg["channel"].(string)

		// Resolve user name and channel name:
		inmsg.UserName = bc.resolveUserName(inmsg.UserID)
		inmsg.ChannelName = bc.resolveChannelName(inmsg.ChannelID)

		log.Printf("#%s (%s) <%s (%s)>::  %s\n",
			inmsg.ChannelName, inmsg.ChannelID,
			inmsg.UserName, inmsg.UserID,
			inmsg.Text,
		)

		// Let the bot do its thing:
		outmsg, werr := botHandleMessage(inmsg)
		if werr != nil {
			log.Printf("  incoming: botHandleMessage ERROR: %s\n", werr)
			break
		}

		// Subdivide message:
		msgs := divideMessage(outmsg.Text, 4000)

		// Send all messages:
		for _, outtext := range msgs {
			params := map[string]string{
				"as_user": "true",
				"channel": inmsg.ChannelID,
				"text":    outtext,
			}
			if outmsg.Attachments != nil {
				b, err := json.Marshal(outmsg.Attachments)
				if err != nil {
					log.Printf("  incoming: JSON marshal error: %s\n", err)
					break
				}
				params["attachments"] = string(b)
			}
			// NOTE(jsd): RTM API does not allow formatted messages or attachments yet.
			_, err := slackAPI("chat.postMessage", params)
			if err != nil {
				log.Printf("  incoming: chat.postMessage error: %s\n", err)
				break
			}
		}

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
		log.Println("  incoming: dying")
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
		bc.handleMessage(&wsInMessage)
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
	if err := parseEnv([]string{"BIT_AUTH", "SLACK_TOKEN", "BOT_USERID"}); err != nil {
		return err
	}

	// BOT_USERID should be something like "U03wxyz", not the bot's user name.
	bot_tag = fmt.Sprintf("<@%s>", env["BOT_USERID"])

	// Start the websocket connector watchdog:
	watchdog()

	return nil
}
