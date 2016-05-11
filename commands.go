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

	cmd := strings.Split(strings.TrimSpace(line), " ")
	ret := ""
	var err error
	switch strings.ToLower(strings.TrimSpace(cmd[0])) {
	case "wtf":
		ret = "Rude.\n"
		fallthrough
	case "help", "?":
		ret += "Press F1 for full help. Use 'quit' or 'exit' to leave."
		config.MainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: ret},
		}
	case "clear":
		var mainView *gocui.View
		mainView, err = config.Interface.View("main")
		if err != nil {
			log.Panic(err)
		}
		mainView.Clear()
	case "start":
		config.closeConnection = startQueue()
	case "stop":
		config.closeConnection <- true
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
			config.MainTextChannel <- WindowMessage{
				Type:    CommandFeedbackMessage,
				Message: zulip.Message{Content: strings.TrimSuffix(ret, "\n")},
			}
		case "save":
			var configYAML []byte
			configYAML, err = yaml.Marshal(config)
			if err != nil {
				config.MainTextChannel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Unable to save config: %s", err)},
				}
				break
			}
			if err = os.MkdirAll(path.Dir(config.ConfigFile), 0755); err != nil {
				config.MainTextChannel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Unable to save config: %s", err)},
				}
				break
			}
			if err = ioutil.WriteFile(config.ConfigFile, configYAML, 0640); err != nil {
				config.MainTextChannel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Unable to save config: %s", err)},
				}
				break
			}
			config.MainTextChannel <- WindowMessage{
				Type:    CommandFeedbackMessage,
				Message: zulip.Message{Content: fmt.Sprintf("Configuration file written to %s", config.ConfigFile)},
			}
		case "set":
			if len(cmd) != 4 {
				goto CmdConfigShowHelp
			}
			err = setConfigFromStrings(strings.ToLower(strings.TrimSpace(cmd[2])), cmd[3])
			if err == nil {
				config.MainTextChannel <- WindowMessage{
					Type:    CommandFeedbackMessage,
					Message: zulip.Message{Content: fmt.Sprintf("%s set to %s", strings.ToLower(strings.TrimSpace(cmd[2])), cmd[3])},
				}
				updateZulipContext()
				break
			}
			config.MainTextChannel <- WindowMessage{
				Type:    ErrorMessage,
				Message: zulip.Message{Content: fmt.Sprintf("Unable to set %s: %v", strings.ToLower(strings.TrimSpace(cmd[2])), err)},
			}
			break
		CmdConfigShowHelp:
			fallthrough
		default:
			ret += "Usage: config show | config set <name> <value> | config save"
			config.MainTextChannel <- WindowMessage{
				Type:    CommandFeedbackMessage,
				Message: zulip.Message{Content: ret},
			}
		}
	case "ping":
		go func(channel chan<- WindowMessage) {
			if err := zulip.CanReachServer(config.zulipContext); err == nil {
				channel <- WindowMessage{
					Type:    CommandFeedbackMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Connection to %s is working properly.", config.zulipContext.APIBase)},
				}
			} else {
				channel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Connection to %s/generate_204 failed: %v.", config.zulipContext.APIBase, err)},
				}
			}
		}(config.MainTextChannel)
	case "priv":
		config.MainTextChannel <- WindowMessage{
			Type:    ErrorMessage,
			Message: zulip.Message{Content: "Not implemented"},
		}
		// zulip.SendPrivateMessage(zulipContext, "Test message! **Hello world!** :octopus: Etc\nand etc.", []string{"test@example.com"}, "Subject for test message")
	case "exit", "quit":
		config.Interface.Execute(func(g *gocui.Gui) error { return gocui.ErrQuit })
	default:
		config.MainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: fmt.Sprintf("Command: %s", cmd[0])},
		}
	}
}
