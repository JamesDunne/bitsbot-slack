package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
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
	breq, err := http.NewRequest("GET", "https://i.bittwiddlers.org/api/v1/list", nil)
	if err != nil {
		return nil, err
	}

	breq.Header.Set("Authorization", os.Getenv("BIT_AUTH"))
	brsp, err := http.DefaultClient.Do(breq)
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
	log.Printf("%v\n", keywords)

	highest, highest_idx := -1, -1
	for idx, img := range list {
		titleLower := strings.ToLower(img.Title)

		words := strings.FieldsFunc(
			titleLower,
			func(c rune) bool { return strings.ContainsRune(" \n\t:,;.[]!()$%^&*/<>'\"", c) },
		)

		h := -2

		// Add points for each keyword match:
		for _, keyword := range keywords {
			if strings.Contains(titleLower, keyword) {
				h += 10
			}
		}

		if h > -2 {
			// Subtract points based on how many extra words there are:
			h += (len(keywords) - len(words))
			if h < 0 {
				h = 0
			}

			log.Printf("  %4d %s\n", h, titleLower)
		}

		if h > highest {
			highest = h
			highest_idx = idx
		}
	}

	if highest_idx == -1 {
		log.Printf("No match!\n")
		return nil
	}

	log.Printf("HIT: %d, %d\n", highest_idx, highest)
	img := list[highest_idx]

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
	log.Printf("%s\n", string(o))
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return nil
	}
	rsp.Write(o)

	return nil
}
