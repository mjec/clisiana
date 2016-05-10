package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/mjec/clisiana/lib/zulip"

	"gopkg.in/yaml.v2"
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
	var err error
	switch strings.ToLower(strings.TrimSpace(cmd[0])) {
	case "wtf":
		ret = "Rude.\n"
		fallthrough
	case "help", "?":
		ret += "Press F1 for full help. Use 'quit' or 'exit' to leave.\n"
		config.MainTextChannel <- ret
	case "clear":
		var mainView *gocui.View
		mainView, err = config.Interface.View("main")
		if err != nil {
			log.Panic(err)
		}
		mainView.Clear()
	case "config":
		reflectedConfig := reflect.ValueOf(config).Elem()
		typeOfReflectedConfig := reflectedConfig.Type()
		if len(cmd) < 2 {
			cmd = []string{cmd[0], ""}
		} else {
			cmd[1] = strings.ToLower(strings.TrimSpace(cmd[1]))
		}
		switch cmd[1] {
		case "show":
			for i := 0; i < reflectedConfig.NumField(); i++ {
				f := reflectedConfig.Field(i)
				fieldName := typeOfReflectedConfig.Field(i).Tag.Get("config-name")
				// NB: Magic constant ("-" for invisible fields)
				if fieldName != "-" {
					// NB: Magic number (12 for width of config-name)
					ret += fmt.Sprintf("%-12s = %v\n", fieldName, f.Interface())
				}
			}
		case "save":
			var configYAML []byte
			configYAML, err = yaml.Marshal(config)
			if err != nil {
				ret = fmt.Sprintf("Unable to save config: %s\n", err)
				break
			}
			if err = os.MkdirAll(path.Dir(config.ConfigFile), 0755); err != nil {
				ret = fmt.Sprintf("Unable to save config: %s\n", err)
				break
			}
			if err = ioutil.WriteFile(config.ConfigFile, configYAML, 0640); err != nil {
				ret = fmt.Sprintf("Unable to save config: %s\n", err)
				break
			}
			ret = fmt.Sprintf("Configuration file written to %s\n", config.ConfigFile)
		case "set":
			if len(cmd) != 4 {
				goto CmdConfigShowHelp
			}
			err = setConfigFromStrings(strings.ToLower(strings.TrimSpace(cmd[2])), cmd[3])
			if err == nil {
				ret += fmt.Sprintf("%s set to %s\n", strings.ToLower(strings.TrimSpace(cmd[2])), cmd[3])
				break
			} else {
				ret += fmt.Sprintf("Error: %s\n", err)
			}
		CmdConfigShowHelp:
			fallthrough
		default:
			ret += "Usage: config show | config set <name> <value> | config save\n"
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
