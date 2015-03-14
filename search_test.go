package main

import (
	"encoding/json"
	"log"
	"os"
	"testing"
)

var bits_list []*ImageViewModel

func Test_keywordSearch(t *testing.T) {
	winners := keywordMatch("is", bits_list)
	if len(winners) == 0 {
		t.Fail()
	}
}

func TestMain(m *testing.M) {
	var err error
	var jsonExists bool

	{
		f, err := os.Open("bits.json")
		if err == nil {
			defer f.Close()
			err = json.NewDecoder(f).Decode(&bits_list)
			if err != nil {
				log.Printf("ERROR: %s\n", err)
				bits_list = nil
			} else {
				jsonExists = true
			}
		}
	}

	if bits_list == nil {
		// Query i.bittwiddlers.org for the list of images:
		if !parseEnv([]string{"BIT_AUTH"}) {
			os.Exit(1)
			return
		}
		bits_list, err = queryBit()
		if err != nil {
			log.Fatalf("ERROR: %s\n", err)
		}
	}

	if !jsonExists {
		f, err := os.OpenFile("bits.json", os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Printf("ERROR: %s\n", err)
		}
		defer f.Close()
		json.NewEncoder(f).Encode(bits_list)
	}

	os.Exit(m.Run())
}
