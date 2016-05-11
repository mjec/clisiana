package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/casimir/xdg-go"
	"github.com/mjec/clisiana/lib/zulip"
)

var config *Config

// DEBUG should only be true during development
var DEBUG = true

func main() {
	config = &Config{}
	config.xdgApp = xdg.App{Name: "clisiana"}

	config.mainTextChannel = make(chan WindowMessage, 5)

	config.cliApp = commandLineSetup()

	config.cliApp.Action = run

	config.cliApp.Run(os.Args)
}

func startQueue() chan bool {
	restartConnection := make(chan bool)
	closeConnection := make(chan bool)
	go func(restartConnection chan bool, closeConnection chan bool, context *zulip.Context) {
		stopGettingEvents := make(chan bool)
		for {
			select {
			case <-closeConnection:
				close(closeConnection)
				stopGettingEvents <- true
				close(stopGettingEvents)
				return
			case <-restartConnection:
				// no-op i.e. loop again
			}
			queueID, lastEventID, err := zulip.Register(context, zulip.MessageEvent, false)
			if err != nil {
				config.mainTextChannel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: fmt.Sprintf("%v", err)},
				}
				break
			}
			go func(queue string,
				lastEventID int64,
				restartConnection chan<- bool,
				stopGettingEvents <-chan bool,
				context *zulip.Context) {
				var err error
				for {
					select {
					case <-stopGettingEvents:
						// We need to do this in case we are not in the middle of a GetEvents request
						// when things are stopped
						break
					default:
						// no-op i.e. loop again
					}
					var events = []zulip.Event{}
					events, err = zulip.GetEvents(context, queue, lastEventID, false)
					if err != nil {
						// Normally this will be because of a cancellation, in which case we just break
						if !strings.HasSuffix(err.Error(), "request canceled") {
							// If it isn't a cancellation, show the error message...
							config.mainTextChannel <- WindowMessage{
								Type:    ErrorMessage,
								Message: zulip.Message{Content: fmt.Sprintf("%v", err)},
							}
							// ...and ask for a new queue, Just In Case
							restartConnection <- true
						}
						break
					}
					for i := range events {
						if events[i].ID > lastEventID {
							lastEventID = events[i].ID
						}
						switch events[i].Type {
						case zulip.HeartbeatEvent:
							break
						case zulip.MessageEvent:
							switch events[i].Message.Type {
							case zulip.StreamMessage:
								config.mainTextChannel <- WindowMessage{
									Type:    StreamMessage,
									Message: events[i].Message,
								}
							case zulip.PrivateMessage:
								config.mainTextChannel <- WindowMessage{
									Type:    PrivateMessage,
									Message: events[i].Message,
								}
							default:
								log.Panic("Unsupported message type: ", events[i].Message.Type)
							}
						default:
							log.Panic("Unsupported event type: ", events[i].Type)
						}
					}
				}
			}(queueID, lastEventID, restartConnection, stopGettingEvents, context)
		}
	}(restartConnection, closeConnection, config.zulipContext)
	restartConnection <- true
	return closeConnection
}
