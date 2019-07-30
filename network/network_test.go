package network

import (
	"log"
	"testing"
)

func TestParseListen(t *testing.T) {
	ip, port, err := ParseListen(":48080")
	log.Println(ip, port, err)
}
