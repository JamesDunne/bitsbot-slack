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
	log.Printf("%d matches\n", len(list))
	for _, img := range list {
		log.Printf("%s\n", img.Title)
	}
}

func TestBitSearchAll(t *testing.T) {
	err := parseEnv([]string{"BIT_AUTH"})
	if err != nil {
		t.Error(err)
		return
	}
	list, err := searchBit([]string{})
	if err != nil {
		t.Error(err)
		return
	}
	log.Printf("%d matches\n", len(list))
	for _, img := range list {
		log.Printf("%s\n", img.Title)
	}
}
