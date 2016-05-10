package main

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/mjec/clisiana/lib/zulip"
)

func parseLine(line string) {
	if line == "" {
		return
	}

	zulipContext := &zulip.Context{
		Email:   config.Email,
		APIKey:  config.APIKey,
		APIBase: config.APIBase,
		Secure:  config.Secure,
	}

	cmd := strings.Split(strings.TrimSpace(line), " ")
	ret := ""
	switch strings.ToLower(strings.TrimSpace(cmd[0])) {
	case "wtf":
		ret = "Rude.\n"
		fallthrough
	case "help", "?":
		ret += "Press F1 for full help. Use 'quit' or 'exit' to leave.\n"
		config.MainTextChannel <- ret
	case "clear":
		mainView, err := config.Interface.View("main")
		if err != nil {
			log.Panic(err)
		}
		mainView.Clear()
	case "config":
		if len(cmd) < 2 {
			cmd = []string{cmd[0], ""}
		} else {
			cmd[1] = strings.ToLower(strings.TrimSpace(cmd[1]))
		}
		switch cmd[1] {
		case "show":
			reflectedConfig := reflect.ValueOf(config).Elem()
			typeOfReflectedConfig := reflectedConfig.Type()
			for i := 0; i < reflectedConfig.NumField(); i++ {
				f := reflectedConfig.Field(i)
				fieldName := typeOfReflectedConfig.Field(i).Tag.Get("config-name")
				shortFieldName := typeOfReflectedConfig.Field(i).Tag.Get("config-short")
				if fieldName != "-" {
					ret += fmt.Sprintf("%s [%s]: %v\n", fieldName, shortFieldName, f.Interface())
				}
			}
		case "set":
			ret = "Not yet implemented"
		case "save":
			ret = "Not yet implemented"
		default:
			ret = "Options: config show | config set <name> <value> | config save\n"
		}
		config.MainTextChannel <- ret
	case "ping":
		go func(channel chan<- string) {
			if err := zulip.CanReachServer(zulipContext); err == nil {
				channel <- fmt.Sprintf("Connection to %s is working properly.\n", zulipContext.APIBase)
			} else {
				channel <- fmt.Sprintf("Connection to %s/generate_204 failed: received %v.\n", zulipContext.APIBase, err)
			}
		}(config.MainTextChannel)
	case "priv":
		config.MainTextChannel <- "Not implemented"
		// zulip.SendPrivateMessage(zulipContext, "Test message! **Hello world!** :octopus: Etc\nand etc.", []string{"test@example.com"}, "Subject for test message")
	case "exit", "quit":
		config.Interface.Execute(func(g *gocui.Gui) error { return gocui.ErrQuit })
	default:
		config.MainTextChannel <- fmt.Sprintf("Command: %s\n", cmd[0])
	}
}
