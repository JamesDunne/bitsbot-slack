package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//import "github.com/JamesDunne/go-util/base"
import "github.com/JamesDunne/go-util/web"

// from i.bittwiddlers.org
type ImageViewModel struct {
	ID             int64   `json:"id"`
	Base62ID       string  `json:"base62id"`
	Title          string  `json:"title"`
	Kind           string  `json:"kind"`
	ImageURL       string  `json:"imageURL"`
	ThumbURL       string  `json:"thumbURL"`
	Submitter      string  `json:"submitter,omitempty"`
	CollectionName string  `json:"collectionName,omitempty"`
	SourceURL      *string `json:"sourceURL,omitempty"`
	RedirectToID   *int64  `json:"redirectToID,omitempty"`
	IsClean        bool    `json:"isClean"`
}

func searchBit(keywords []string) (list []*ImageViewModel, err error) {
	// Query i.bittwiddlers.org
	requrl := "https://i.bittwiddlers.org/api/v1/search/all?q=" + url.QueryEscape(strings.Join(keywords, " "))
	breq, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		return nil, err
	}

	breq.Header.Set("Authorization", env["BIT_AUTH"])

	// Skip bad self-signed x509 certificate:
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	brsp, err := client.Do(breq)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return nil, err
	}

	list = make([]*ImageViewModel, 0, 1000)
	err = json.NewDecoder(brsp.Body).Decode(
		&struct {
			Result struct {
				List *[]*ImageViewModel `json:"list"`
			} `json:"result"`
		}{
			Result: struct {
				List *[]*ImageViewModel `json:"list"`
			}{
				List: &list,
			},
		},
	)

	return list, err
}

func textReply(text string) *SlackOutMessage {
	return &SlackOutMessage{
		Text: text,
	}
}

// Runes used to split words:
const wordSplitters = " \n\t:,;.-+=[]!?()$%^&*<>\"`"

// Split a string into separate words:
func splitToWords(text string) []string {
	return strings.FieldsFunc(
		text,
		func(c rune) bool { return strings.ContainsRune(wordSplitters, c) },
	)
}

func botHandleMessage(msg *SlackInMessage) (*SlackOutMessage, *web.Error) {
	text := msg.Text

	// -list prefix will list best matches instead of randomly selecting one:
	do_list := false
	if strings.HasPrefix(text, "-list") {
		do_list = true
		text = strings.TrimLeft(text[len("-list"):], " :\t\n")
	}

	// Query i.bittwiddlers.org for the list of images:
	keywords := splitToWords(text)
	list, err := searchBit(keywords)
	if err != nil {
		log.Printf("bittwiddlers search ERROR: %s\n", err)
		return nil, nil
	}

	// Filter list into images only; no youtube or gifv:
	winners := make([]*ImageViewModel, 0, len(list))
	for _, img := range list {
		if img.Kind != "gif" && img.Kind != "jpeg" && img.Kind != "png" {
			continue
		}
		winners = append(winners, img)
	}

	if do_list {
		out := new(bytes.Buffer)
		if len(winners) == 0 {
			fmt.Fprintf(out, "No matches for '%s'.\n", text)
		} else {
			fmt.Fprintf(out, "Best matches for '%s':\n", text)
			for _, img := range winners {
				fmt.Fprintf(out, " * <http://i.bittwiddlers.org/b/%s|%s>\n", img.Base62ID, img.Title)
			}
		}
		return textReply(out.String()), nil
	}

	img := (*ImageViewModel)(nil)
	if len(winners) == 0 {
		log.Printf("  No match!\n")
	} else if len(winners) == 1 {
		img = winners[0]
	} else {
		log.Printf("  %d winners; randomly selecting a winner\n", len(winners))
		for _, img := range winners {
			log.Printf("    %s: %s\n", img.Base62ID, img.Title)
		}

		// Initialize a pseudo-random source:
		timestamp := int64(0)
		timestamp_float, err := strconv.ParseFloat(msg.Timestamp, 64)
		if err != nil {
			timestamp = time.Now().UnixNano()
		} else {
			timestamp = int64(timestamp_float)
		}

		// Select a random winner:
		r := rand.New(rand.NewSource(timestamp))
		img = winners[r.Intn(len(winners))]
	}

	if img == nil {
		return textReply(fmt.Sprintf("Sorry, %s, no match for '%s'.", msg.UserName, text)), nil
	}

	log.Printf("  winner: %s: %s\n", img.Base62ID, img.Title)

	return &SlackOutMessage{
		Text: img.Title,
		Attachments: []SlackMessageAttachment{
			SlackMessageAttachment{
				Fallback: img.Title,
				ImageURL: "http://i.bittwiddlers.org" + img.ImageURL,
			},
		},
	}, nil
}
