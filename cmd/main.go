package main

import (
	"log"
	"time"

	mv370 "github.com/kgolding/go-mv-370"
)

const timeout = 10 * time.Second

func main() {
	host := "192.168.100.251:23"

	log.Printf("Host: %s", host)
	c, err := mv370.New(host, "voip", "1234")
	if err != nil {
		panic(err)
	}

	// log.Printf("Sending SMS")
	// err = c.SendSMS("07816971716", "Hello World!\nAn extra line\n- or two! :)")
	// if err != nil {
	// 	panic(err)
	// }

	log.Printf("Read SMS")
	messages, err := c.ReadSMS()
	if err != nil {
		panic(err)
	}
	for i, msg := range messages {
		log.Printf("Message %d, from '%s', date: '%s': '%s'", i, msg.Tel, msg.Time, msg.Message)
	}
}
