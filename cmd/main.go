package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	mv370 "github.com/kgolding/go-mv-370"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "enable debug output")

	var host, username, password string
	flag.StringVar(&host, "host", "192.168.100.251:23", "host IP and port")
	flag.StringVar(&username, "user", "voip", "MV370 username")
	flag.StringVar(&password, "pass", "1234", "MV370 password")

	flag.Parse()

	var tel, text string
	args := flag.Args()

	switch len(args) {
	case 0:
		// No args, so we'll read the messages
	case 1:
		// One arg, which should be a json object with tel & text fields
		var msg mv370.Message
		err := json.Unmarshal([]byte(args[0]), &msg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(2)
		}
		tel = msg.Tel
		text = msg.Text
	default:
		flag.Usage()
		fmt.Fprintf(os.Stderr, "Examples:\n  - %s\n        to read messages\n  - %s '{\"tel\":\"012345678\",\"text\":\"The message\"}'\n        to send a message\n",
			os.Args[0], os.Args[0])
		os.Exit(1)
	}

	var logger *slog.Logger

	if debug {
		opts := &slog.HandlerOptions{
			Level: slog.LevelDebug,
			// Level: slog.LevelInfo,
		}
		logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
	}

	c, err := mv370.New(host, username, password, logger)
	if err != nil {
		panic(err)
	}

	if tel != "" && text != "" {
		// Send a message
		err = c.SendSMS(tel, text)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(4)
		}
		os.Exit(0)
	} else {
		// Read messages and return as a json array
		messages, err := c.ReadSMS()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		b, _ := json.Marshal(messages)
		fmt.Println(string(b))
		os.Exit(0)
	}
}
