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

func parseCmdLine(line string) {
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
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: ret},
		}
	case "clear":
		var mainView *gocui.View
		mainView, err = config.ui.View("main")
		if err != nil {
			log.Panic("Cannot clear: " + err.Error())
		}
		mainView.Clear()
	case "disconnect":
		config.closeConnection <- true // this channel will be closed by its receiver
		zulip.CancelAllRequests()
		config.closeConnection = nil
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: fmt.Sprintf("Disconnected from %s", config.zulipContext.APIBase)},
		}
	case "config":
		handleCommandConfig(cmd)
	case "private":
		results := showNewPrivateMessagePrompt(config.ui, zulip.OutgoingPrivateMessage{})
		go func(channel chan<- WindowMessage, result <-chan struct {
			zulip.OutgoingPrivateMessage
			error
		}) {
			r := <-result
			if r.error != nil {
				channel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Unable to get a new private message: %v.", r.error)},
				}
				return
			}
			msgid, err := zulip.SendPrivateMessage(config.zulipContext, r.OutgoingPrivateMessage)
			if err != nil {
				channel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: err.Error()},
				}
			} else {
				channel <- WindowMessage{
					Type:    DebugMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Private message sent: got message ID %d", msgid)},
				}
			}
		}(config.mainTextChannel, results)
	case "stream":
		results := showNewStreamMessagePrompt(config.ui, zulip.OutgoingStreamMessage{
			Stream: "test-stream",
			Topic:  "Testing clisiana",
		})
		go func(channel chan<- WindowMessage, result <-chan struct {
			zulip.OutgoingStreamMessage
			error
		}) {
			r := <-result
			if r.error != nil {
				channel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Unable to get a new stream message: %v.", r.error)},
				}
				return
			}
			msgid, err := zulip.SendStreamMessage(config.zulipContext, r.OutgoingStreamMessage)
			if err != nil {
				channel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: err.Error()},
				}
			} else {
				channel <- WindowMessage{
					Type:    DebugMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Stream message sent: got message ID %d", msgid)},
				}
			}
		}(config.mainTextChannel, results)
	case "connect":
		config.closeConnection = startReceivingMessages()
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: fmt.Sprintf("Okay, waiting for messages from %s", config.zulipContext.APIBase)},
		}
	case "ping":
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: fmt.Sprintf("Attempting to ping %s", config.APIBase)},
		}
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
		}(config.mainTextChannel)
	case "exit", "quit":
		config.ui.Execute(func(g *gocui.Gui) error { return gocui.ErrQuit })
	case "testmsg":
		if DEBUG {
			if len(cmd) != 2 {
				config.mainTextChannel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: "Exactly one email address must be specified"},
				}
				break
			}
			config.mainTextChannel <- WindowMessage{
				Type:    CommandFeedbackMessage,
				Message: zulip.Message{Content: fmt.Sprintf("Attempting to send test message to %s", cmd[1])},
			}
			go func(channel chan<- WindowMessage) {
				msgid, err := zulip.SendPrivateMessage(config.zulipContext,
					zulip.OutgoingPrivateMessage{
						To:      []string{cmd[1]},
						Content: "Test message!\n**Hello world** is in bold.\n:octopus: is an emoji.",
					})
				if err != nil {
					channel <- WindowMessage{
						Type:    ErrorMessage,
						Message: zulip.Message{Content: err.Error()},
					}
				} else {
					channel <- WindowMessage{
						Type:    DebugMessage,
						Message: zulip.Message{Content: fmt.Sprintf("Test message sent: got message ID %d", msgid)},
					}
				}
			}(config.mainTextChannel)
			break
		}
		fallthrough
	default:
		config.mainTextChannel <- WindowMessage{
			Type:    ErrorMessage,
			Message: zulip.Message{Content: fmt.Sprintf("Command does not exist: %s", cmd[0])},
		}
	}
}

func handleCommandConfig(cmd []string) {
	var err error
	ret := ""
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
			if fieldName != "" && fieldName != "-" {
				// NB: Magic number (12 for width of config-name)
				ret += fmt.Sprintf("%-12s = %v\n", fieldName, f.Interface())
			}
		}
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: strings.TrimSuffix(ret, "\n")},
		}
	case "save":
		var configYAML []byte
		configYAML, err = yaml.Marshal(config)
		if err != nil {
			config.mainTextChannel <- WindowMessage{
				Type:    ErrorMessage,
				Message: zulip.Message{Content: fmt.Sprintf("Unable to save config: %s", err)},
			}
			break
		}
		if err = os.MkdirAll(path.Dir(config.ConfigFile), 0755); err != nil {
			config.mainTextChannel <- WindowMessage{
				Type:    ErrorMessage,
				Message: zulip.Message{Content: fmt.Sprintf("Unable to save config: %s", err)},
			}
			break
		}
		if err = ioutil.WriteFile(config.ConfigFile, configYAML, 0640); err != nil {
			config.mainTextChannel <- WindowMessage{
				Type:    ErrorMessage,
				Message: zulip.Message{Content: fmt.Sprintf("Unable to save config: %s", err)},
			}
			break
		}
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: fmt.Sprintf("Configuration file written to %s", config.ConfigFile)},
		}
	case "set":
		if len(cmd) != 4 {
			goto CmdConfigShowHelp
		}
		err = setConfigFromStrings(strings.ToLower(strings.TrimSpace(cmd[2])), cmd[3])
		if err == nil {
			config.mainTextChannel <- WindowMessage{
				Type:    CommandFeedbackMessage,
				Message: zulip.Message{Content: fmt.Sprintf("%s set to %s", strings.ToLower(strings.TrimSpace(cmd[2])), cmd[3])},
			}
			updateZulipContext()
			break
		}
		config.mainTextChannel <- WindowMessage{
			Type:    ErrorMessage,
			Message: zulip.Message{Content: fmt.Sprintf("Unable to set %s: %v", strings.ToLower(strings.TrimSpace(cmd[2])), err)},
		}
		break
	CmdConfigShowHelp:
		fallthrough
	default:
		ret += "Usage: config show | config set <name> <value> | config save"
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: ret},
		}
	}
}
