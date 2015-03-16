package main

import (
	"log"
	"testing"
)

func TestDivideMessage(t *testing.T) {
	k := "abcd\nefghijklmnopqrstuvwxyz"
	msgs := divideMessage(k, 13)
	log.Println(msgs)
}
