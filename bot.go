package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
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

func queryBit() (list []*ImageViewModel, err error) {
	// Query i.bittwiddlers.org
	breq, err := http.NewRequest("GET", "https://i.bittwiddlers.org/api/v1/all", nil)
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

func keywordMatch(text string, list []*ImageViewModel) (winners []*ImageViewModel) {
	// No keywords means match all:
	if text == "" {
		return list
	}

	// Runes used to split words:
	const wordSplitters = " \n\t:,;.-+=[]!?()$%^&*<>\"`"

	// Search by keyword:
	keywords := strings.FieldsFunc(
		strings.ToLower(text),
		func(c rune) bool { return strings.ContainsRune(wordSplitters, c) },
	)
	log.Printf("  keywords: %v\n", keywords)

	highest := -1
	highest_idxs := make([]int, 0, 20)
	for idx, img := range list {
		titleLower := strings.ToLower(img.Title)

		words := strings.FieldsFunc(
			titleLower,
			func(c rune) bool { return strings.ContainsRune(wordSplitters, c) },
		)

		h := -2

		// Add points for each keyword match:
		last_word_idx := -1
		// TODO(jsd): Don't count single-word matches on useless filler words like articles; only count them if in a phrase.
		// TODO(jsd): Prefer to match all keywords.
		for _, keyword := range keywords {
			found := false
			for word_idx, word := range words {
				if word == keyword {
					found = true

					if last_word_idx > -1 {
						if word_idx > last_word_idx+1 {
							// Penalize distance (word count) from last word found (helps phrases match better):
							h -= ((word_idx - last_word_idx) + 1)
						}
					}

					h += 10
					h = (h * 20) / 16
					last_word_idx = word_idx

					// Only trigger once per keyword:
					break
				}
			}

			// All keywords are required to match:
			if !found {
				h = -2
				break
			}
		}

		if h > -2 {
			log.Printf("  %4d %s\n", h, img.Title)

			if h > highest {
				highest = h
				// Build a new list of winners at this index:
				highest_idxs = highest_idxs[:0]
				highest_idxs = append(highest_idxs, idx)
			} else if h == highest {
				// Add to the winning pool:
				highest_idxs = append(highest_idxs, idx)
			}
		}
	}

	winners = make([]*ImageViewModel, 0, len(highest_idxs))
	for _, idx := range highest_idxs {
		winners = append(winners, list[idx])
	}
	return
}

func textReply(text string) *SlackOutMessage {
	return &SlackOutMessage{
		Text: text,
	}
}

func botHandleMessage(msg *SlackInMessage) (*SlackOutMessage, *web.Error) {
	// Query i.bittwiddlers.org for the list of images:
	list, err := queryBit()
	if err != nil {
		log.Printf("bittwiddlers query ERROR: %s\n", err)
		return nil, nil
	}

	// Filter list into images only; no youtube or gifv:
	img_list := make([]*ImageViewModel, 0, len(list))
	for _, img := range list {
		if img.Kind != "gif" && img.Kind != "jpeg" && img.Kind != "png" {
			continue
		}
		img_list = append(img_list, img)
	}

	text := msg.Text

	// -list prefix will list best matches instead of randomly selecting one:
	do_list := false
	if strings.HasPrefix(text, "-list") {
		do_list = true
		text = strings.TrimLeft(text[len("-list"):], " :\t\n")
	}

	// Keyword search through image titles and find best matches:
	winners := keywordMatch(text, img_list)

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
		log.Printf("  Single match!\n")
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
