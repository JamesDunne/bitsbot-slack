package main

import (
	"log"
	"testing"
)

func TestBitSearch(t *testing.T) {
	err := parseEnv([]string{"BIT_AUTH"})
	if err != nil {
		t.Error(err)
		return
	}
	list, err := searchBit([]string{"jack", "nicholson"})
	if err != nil {
		t.Error(err)
		return
	}
	for _, img := range list {
		log.Printf("%s\n", img.Title)
	}
}
