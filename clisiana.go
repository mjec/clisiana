package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/casimir/xdg-go"
	"github.com/mjec/clisiana/lib/notifications"
	"github.com/mjec/clisiana/lib/zulip"
)

var config *Config

// DEBUG should only be true during development
var DEBUG = false

func main() {
	if !DEBUG {
		log.SetOutput(ioutil.Discard)
	}
	config = &Config{}
	config.xdgApp = xdg.App{Name: "clisiana"}

	config.mainTextChannel = make(chan WindowMessage, 5)
	config.outgoingStreamMessagesChannel = make(chan zulip.OutgoingStreamMessage, 10)
	config.outgoingPrivateMessagesChannel = make(chan zulip.OutgoingPrivateMessage, 10)

	config.cliApp = commandLineSetup()

	config.cliApp.Action = run

	config.cliApp.Run(os.Args)
}

func startReceivingMessages() chan bool {
	restartConnection := make(chan bool)
	closeConnection := make(chan bool)
	go func(restartConnection chan bool, closeConnection chan bool, zulipContext *zulip.Context) {
		stopGettingEvents := make(chan bool)
		for {
			select {
			case <-closeConnection:
				config.mainTextChannel <- WindowMessage{
					Type:    DebugMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Closing connection...")},
				}
				close(closeConnection)
				stopGettingEvents <- true
				close(stopGettingEvents)
				return
			case <-restartConnection:
				// no-op i.e. loop again
			}
			queueID, lastEventID, err := zulip.Register(zulipContext, zulip.MessageEvent, false)
			if err != nil {
				config.mainTextChannel <- WindowMessage{
					Type:    ErrorMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Cannot register: %v", err)},
				}
				break
			} else {
				config.mainTextChannel <- WindowMessage{
					Type:    DebugMessage,
					Message: zulip.Message{Content: fmt.Sprintf("Queue %s obtained, waiting for messages...", queueID)},
				}
			}
			go func(queue string,
				lastEventID int64,
				restartConnection chan<- bool,
				stopGettingEvents <-chan bool,
				zulipContext *zulip.Context) {
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
					events, err = zulip.GetEvents(zulipContext, queue, lastEventID, false)
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
								config.notifications.Push(notifications.Notification{
									Title:   fmt.Sprintf("%s > %s", events[i].Message.DisplayRecipient.Stream, events[i].Message.Subject),
									Content: events[i].Message.Content,
								})
								config.mainTextChannel <- WindowMessage{
									Type:    StreamMessage,
									Message: events[i].Message,
								}
							case zulip.PrivateMessage:
								config.notifications.Push(notifications.Notification{
									Title:   fmt.Sprintf("Private message from %s", events[i].Message.SenderFullName),
									Content: events[i].Message.Content,
								})
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
			}(queueID, lastEventID, restartConnection, stopGettingEvents, zulipContext)
		}
	}(restartConnection, closeConnection, config.zulipContext)
	restartConnection <- true
	return closeConnection
}
