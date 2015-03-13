package main

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//import "github.com/JamesDunne/go-util/base"
import "github.com/JamesDunne/go-util/web"

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
	json.NewDecoder(brsp.Body).Decode(
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

	return list, nil
}

func replyText(rsp http.ResponseWriter, text string) {
	json.NewEncoder(rsp).Encode(struct {
		Text string `json:"text"`
	}{
		Text: text,
	})
}

func processRequest(rsp http.ResponseWriter, req *http.Request) *web.Error {
	if err := req.ParseForm(); err != nil {
		log.Printf("Could not parse form: %s\n", err)
		return web.AsError(err, http.StatusBadRequest)
	}

	if req.PostForm.Get("token") != env["SLACK_TOKEN"] {
		// Not meant for us.
		rsp.WriteHeader(http.StatusOK)
		return nil
	}

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
	// Query i.bittwiddlers.org for the list of images:
	list, err := queryBit()
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return nil
	}

	// Search by keyword:
	keywords := strings.FieldsFunc(
		strings.ToLower(text),
		func(c rune) bool { return strings.ContainsRune(" \n\t:,;.[]!()$%^&*/<>'\"", c) },
	)
	//log.Printf("  keywords: %v\n", keywords)

	highest := -1
	highest_idxs := make([]int, 0, 20)
	for idx, img := range list {
		if img.Kind != "gif" && img.Kind != "jpeg" && img.Kind != "png" {
			continue
		}

		titleLower := strings.ToLower(img.Title)

		words := strings.FieldsFunc(
			titleLower,
			func(c rune) bool { return strings.ContainsRune(" \n\t:,;.[]!()$%^&*/<>'\"", c) },
		)

		h := -2

		// Add points for each keyword match:
		last_word_idx := -1
		for _, keyword := range keywords {
			for word_idx, word := range words {
				if word == keyword {
					h += 10
					if last_word_idx > -1 {
						// Penalize distance (word count) from last word found (helps phrases match better):
						h -= ((word_idx - last_word_idx) + 1)
					}
					h = (h * 20) / 16
					last_word_idx = word_idx

					// Only trigger once per keyword:
					break
				}
			}
		}

		if h > -2 {
			//log.Printf("  %4d %s\n", h, img.Title)

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

	winning_idx := -1
	if len(highest_idxs) == 0 {
		//log.Printf("  No match!\n")
		replyText(rsp, "No match")
		return nil
	} else if len(highest_idxs) == 1 {
		//log.Printf("  Single match!\n")
		winning_idx = highest_idxs[0]
	} else {
		//log.Printf("  %d winners at %d score; randomly selecting a winner\n", len(highest_idxs), highest)
		//for _, idx := range highest_idxs {
		//	img := list[idx]
		//	log.Printf("    %s\n", img.Title)
		//}

		// Initialize a pseudo-random source:
		timestamp := int64(0)
		timestamp_float, err := strconv.ParseFloat(req.PostFormValue("timestamp"), 64)
		if err != nil {
			timestamp = time.Now().UnixNano()
		} else {
			timestamp = int64(timestamp_float)
		}

		// Select a random winner:
		r := rand.New(rand.NewSource(timestamp))
		winning_idx = highest_idxs[r.Intn(len(highest_idxs))]
	}

	img := list[winning_idx]
	//log.Printf("  %s\n", img.Title)

	// Write JSON response with attached image:
	rsp.Header().Set("Content-Type", "application/json")
	rsp.WriteHeader(http.StatusOK)
	o, err := json.Marshal(struct {
		Text        string `json:"text"`
		Attachments []struct {
			Fallback string `json:"fallback"`
			ImageURL string `json:"image_url"`
		} `json:"attachments"`
	}{
		Text: img.Title,
		Attachments: []struct {
			Fallback string `json:"fallback"`
			ImageURL string `json:"image_url"`
		}{
			struct {
				Fallback string `json:"fallback"`
				ImageURL string `json:"image_url"`
			}{
				Fallback: img.Title,
				ImageURL: "http://i.bittwiddlers.org" + img.ImageURL,
			},
		},
	})
	//log.Printf("%s\n", string(o))
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return nil
	}
	rsp.Write(o)

	return nil
}
