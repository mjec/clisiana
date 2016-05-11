package main

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/casimir/xdg-go"
	"github.com/mjec/clisiana/lib/zulip"
)

var config *Config
var DEBUG = true

func main() {
	if DEBUG {
		f, _ := os.Create("/tmp/clisiana-cpu-profile.prof")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	config = &Config{}
	config.XDGApp = xdg.App{Name: "clisiana"}

	config.MainTextChannel = make(chan WindowMessage, 5)

	config.CLIApp = commandLineSetup()
	config.CLIApp.Action = run

	config.CLIApp.Run(os.Args)
}

func startQueue() chan bool {
	restartConnection := make(chan bool)
	closeConnection := make(chan bool)
	go func(restartConnection chan bool, closeConnection chan bool, context *zulip.Context) {
		stopGettingEvents := make(chan bool)
		for {
			select {
			case <-closeConnection:
				config.MainTextChannel <- WindowMessage{
					Type:    DebugMessage,
					Message: zulip.Message{Content: "Closing queue register goroutine..."},
				}
				close(closeConnection)
				stopGettingEvents <- true
				close(stopGettingEvents)
				return
			case <-restartConnection:
				// no-op i.e. loop again
			}
			config.MainTextChannel <- WindowMessage{
				Type:    DebugMessage,
				Message: zulip.Message{Content: "Running register..."},
			}
			queueID, lastEventID, err := zulip.Register(context, zulip.MessageEvent, false)
			if err != nil {
				config.MainTextChannel <- WindowMessage{
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
						config.MainTextChannel <- WindowMessage{
							Type:    DebugMessage,
							Message: zulip.Message{Content: "Closing get events goroutine..."},
						}
						return
					default:
						// no-op i.e. loop again
					}
					var events = []zulip.Event{}
					config.MainTextChannel <- WindowMessage{
						Type:    DebugMessage,
						Message: zulip.Message{Content: "Getting events..."},
					}
					events, err = zulip.GetEvents(context, queue, lastEventID, false)
					if err != nil {
						config.MainTextChannel <- WindowMessage{
							Type:    ErrorMessage,
							Message: zulip.Message{Content: fmt.Sprintf("%v", err)},
						}
						restartConnection <- true
						break
					}
					config.MainTextChannel <- WindowMessage{
						Type:    DebugMessage,
						Message: zulip.Message{Content: fmt.Sprintf("Got %d event(s)...", len(events))},
					}
					for i := range events {
						if events[i].ID > lastEventID {
							lastEventID = events[i].ID
						}
						switch events[i].Type {
						case zulip.HeartbeatEvent:
							config.MainTextChannel <- WindowMessage{
								Type:    DebugMessage,
								Message: zulip.Message{Content: "Heartbeat received..."},
							}
							break
						case zulip.MessageEvent:
							switch events[i].Message.Type {
							case zulip.StreamMessage:
								config.MainTextChannel <- WindowMessage{
									Type:    StreamMessage,
									Message: events[i].Message,
								}
							case zulip.PrivateMessage:
								config.MainTextChannel <- WindowMessage{
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
		config.MainTextChannel <- WindowMessage{
			Type:    DebugMessage,
			Message: zulip.Message{Content: "Connection closed..."},
		}
	}(restartConnection, closeConnection, config.zulipContext)
	restartConnection <- true
	return closeConnection
}
