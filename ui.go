package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/codegangsta/cli"
	"github.com/jroimartin/gocui"
	"github.com/mjec/clisiana/lib/notifications"
	"github.com/mjec/clisiana/lib/zulip"
)

// WindowMessageType is an enum of messages which may be sent to the main window
type WindowMessageType int

// WindowMessageType possibilities
const (
	PrivateMessage         WindowMessageType = iota // PrivateMessage represents a private message sent to a window
	StreamMessage          WindowMessageType = iota // StreamMessage represents a private message sent to a window
	ErrorMessage           WindowMessageType = iota // ErrorMessage represents an error sent to a window
	CommandFeedbackMessage WindowMessageType = iota // CommandFeedbackMessage represents non-error command feedback sent to a window
	DebugMessage           WindowMessageType = iota // DebugMessage represents a debug message (normally not shown)
)

// WindowMessage is a struct for messages sent to the main window
type WindowMessage struct {
	Type    WindowMessageType
	Message zulip.Message
}

func run(c *cli.Context) error {
	// Normalise API base to not terminate with /
	config.APIBase = strings.TrimSuffix(config.APIBase, "/")

	// Make directories for files if necessary
	os.MkdirAll(path.Dir(config.CacheFile), 0755)

	if config.Logging {
		os.MkdirAll(path.Dir(config.LogFile), 0755)
	}

	if config.NotificationsEnabled {
		config.notifications = notifications.OSAppropriateNotifier()
	} else {
		config.notifications = notifications.DummyNotifier()
	}

	// Set up interface
	config.ui = gocui.NewGui()
	if err := config.ui.Init(); err != nil {
		log.Panicln(err)
	}
	defer config.ui.Close()
	config.ui.Cursor = true

	config.ui.SetLayout(layout)

	if err := setGlobalKeybindings(config.ui); err != nil {
		log.Panicln(err)
	}

	// We can't guarantee this will run FIFO, but it should only matter
	// when things are added very quickly one after the other because
	// this is an unbuffered channel.
	go func(mainWindow <-chan WindowMessage, ifce *gocui.Gui) {
		for msg := range mainWindow {
			// Execute() does not run immediately but gets added
			// to the user events queue. Again, this makes us one
			// step further away from FIFO.
			ifce.Execute(makeMainViewUpdater(msg))
		}
	}(config.mainTextChannel, config.ui)

	go func(messages <-chan zulip.OutgoingStreamMessage, channel chan<- WindowMessage) {
		for msg := range messages {
			msgid, err := zulip.SendStreamMessage(config.zulipContext, msg)
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
		}
	}(config.outgoingStreamMessagesChannel, config.mainTextChannel)

	go func(messages <-chan zulip.OutgoingPrivateMessage, channel chan<- WindowMessage) {
		for msg := range messages {
			msgid, err := zulip.SendPrivateMessage(config.zulipContext, msg)
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
		}
	}(config.outgoingPrivateMessagesChannel, config.mainTextChannel)

	config.ui.Execute(func(g *gocui.Gui) error {
		return g.SetCurrentView("cmd")
	})

	if err := config.ui.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

	return nil
}

// makeMainViewUpdater returns a function which can be passed to gocui.Gui.Execute()
// which will update the main view to display the WindowMessage.
func makeMainViewUpdater(m WindowMessage) gocui.Handler {
	str := m.Message.Content
	switch m.Type {
	case CommandFeedbackMessage:
		str = fmt.Sprintf("CMD: %s\n", str)
	case DebugMessage:
		if !DEBUG {
			return func(g *gocui.Gui) error { return nil }
		}
		str = fmt.Sprintf("DEBUG: %s\n", str)
	case ErrorMessage:
		str = fmt.Sprintf("ERROR: %s\n", str)
	case PrivateMessage:
		str = fmt.Sprintf("\n%s Private messge from %s\n%s\n\n", config.Prompt, m.Message.SenderFullName, str)
	case StreamMessage:
		str = fmt.Sprintf("\n%s Stream messge to %s from %s\n%s\n\n", config.Prompt, m.Message.DisplayRecipient.Stream, m.Message.SenderFullName, str)
	default:
		str = fmt.Sprintf("%s\n", str)
	}

	return func(g *gocui.Gui) error {
		main, err := g.View("main")
		if err != nil {
			return err
		}
		fmt.Fprint(main, str)
		return nil
	}
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	main, err := g.SetView("main", -1, -1, maxX, maxY-3)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	main.Autoscroll = true
	main.Wrap = true

	sizeOfPrompt := utf8.RuneCountInString(config.Prompt)

	cmd, err := g.SetView("cmd", sizeOfPrompt, maxY-3, maxX, maxY)
	cmd.Frame = false
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	cmd.Editable = true

	promptView, err := g.SetView("prompt", -1, maxY-3, sizeOfPrompt, maxY)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	promptView.Frame = false
	switch strings.ToLower(strings.TrimSpace(config.PromptColor)) {
	case "black":
		promptView.FgColor = gocui.ColorBlack
	case "green":
		promptView.FgColor = gocui.ColorGreen
	case "yellow":
		promptView.FgColor = gocui.ColorYellow
	case "blue":
		promptView.FgColor = gocui.ColorBlue
	case "magenta":
		promptView.FgColor = gocui.ColorMagenta
	case "cyan":
		promptView.FgColor = gocui.ColorCyan
	case "white":
		promptView.FgColor = gocui.ColorWhite
	case "red":
		promptView.FgColor = gocui.ColorRed
	case "none":
		break
	}
	promptView.Clear()
	fmt.Fprint(promptView, config.Prompt)

	curView := g.CurrentView()
	if curView != nil {
		switch g.CurrentView().Name() {
		case "cmd":
			g.Editor = gocui.EditorFunc(cuiCmdEditor)
		case "stream-view-stream":
			g.Editor = gocui.EditorFunc(cuiStreamMessageStreamEditor)
		case "stream-view-topic":
			g.Editor = gocui.EditorFunc(cuiStreamMessageTopicEditor)
		case "stream-view-content":
			g.Editor = gocui.EditorFunc(cuiStreamMessageContentEditor)
		// case "private-view-content":
		default:
			g.Editor = gocui.DefaultEditor
		}
	} else {
		g.Editor = gocui.DefaultEditor
	}

	return nil
}

// TODO: implement
func showNewPrivateMessagePrompt(g *gocui.Gui, initialMessage zulip.OutgoingPrivateMessage) chan struct {
	zulip.OutgoingPrivateMessage
	error
} {
	nullMsg := zulip.OutgoingPrivateMessage{} // returned if this function errors before launching goroutine
	privateMessageSendResultChannel := make(chan struct {
		zulip.OutgoingPrivateMessage
		error
	})
	g.Execute(func(g *gocui.Gui) error {
		privateMessageSendResultChannel <- struct {
			zulip.OutgoingPrivateMessage
			error
		}{nullMsg, fmt.Errorf("Not implemented")}
		return nil
	})
	return privateMessageSendResultChannel
}

func sendStreamMessageFromPrompt() {
	// TODO: improve error handling
	var streamView, topicView, contentView *gocui.View
	var err error
	var stream, topic, content string
	streamView, err = config.ui.View("stream-view-stream")
	if streamView == nil || err != nil {
		config.mainTextChannel <- WindowMessage{
			Type:    ErrorMessage,
			Message: zulip.Message{Content: "Problem with stream-view-stream"},
		}
		return
	}
	stream = streamView.Buffer()
	topicView, err = config.ui.View("stream-view-topic")
	if topicView == nil || err != nil {
		config.mainTextChannel <- WindowMessage{
			Type:    ErrorMessage,
			Message: zulip.Message{Content: "Problem with stream-view-stream"},
		}
		return
	}
	topic = topicView.Buffer()
	contentView, err = config.ui.View("stream-view-content")
	if contentView == nil || err != nil {
		config.mainTextChannel <- WindowMessage{
			Type:    ErrorMessage,
			Message: zulip.Message{Content: "Problem with stream-view-stream"},
		}
		return
	}
	content = contentView.Buffer()

	if stream == "" {
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: "You must specify a stream!"},
		}
		return
	}

	if content == "" {
		config.mainTextChannel <- WindowMessage{
			Type:    CommandFeedbackMessage,
			Message: zulip.Message{Content: "You must specify some content!"},
		}
		return
	}

	config.outgoingStreamMessagesChannel <- zulip.OutgoingStreamMessage{
		Stream:  stream,
		Topic:   topic,
		Content: content,
	}
}

func destroyStreamMessagePrompt() {
	config.ui.Execute(func(g *gocui.Gui) error {
		var err error
		if err = g.DeleteView("stream-view-stream"); err != nil {
			log.Panic(err)
		}
		if err = g.DeleteView("stream-view-topic"); err != nil {
			log.Panic(err)
		}
		if err = g.DeleteView("stream-view-content"); err != nil {
			log.Panic(err)
		}
		if err = g.SetCurrentView("cmd"); err != nil {
			log.Panic(err)
		}
		return nil
	})
}

func showNewStreamMessagePrompt(g *gocui.Gui, initialMessage zulip.OutgoingStreamMessage) chan struct {
	zulip.OutgoingStreamMessage
	error
} {
	nullMsg := zulip.OutgoingStreamMessage{} // returned if this function errors before launching goroutine
	streamMessageSendResultChannel := make(chan struct {
		zulip.OutgoingStreamMessage
		error
	})

	g.Execute(func(g *gocui.Gui) error {
		maxX, maxY := g.Size()
		var streamView, topicView, contentView *gocui.View
		var err error
		if streamView, err = g.SetView("stream-view-stream", maxX/2-30, maxY/2-1, maxX/2, maxY/2+1); err != nil {
			if err != gocui.ErrUnknownView {
				streamMessageSendResultChannel <- struct {
					zulip.OutgoingStreamMessage
					error
				}{nullMsg, err}
				return err
			}
		}
		if topicView, err = g.SetView("stream-view-topic", maxX/2, maxY/2-1, maxX/2+30, maxY/2+1); err != nil {
			if err != gocui.ErrUnknownView {
				if err != gocui.ErrUnknownView {
					streamMessageSendResultChannel <- struct {
						zulip.OutgoingStreamMessage
						error
					}{nullMsg, err}
					return err
				}
			}
		}
		if contentView, err = g.SetView("stream-view-content", maxX/2-30, maxY/2+2, maxX/2+30, maxY/2+10); err != nil {
			if err != gocui.ErrUnknownView {
				if err != gocui.ErrUnknownView {
					streamMessageSendResultChannel <- struct {
						zulip.OutgoingStreamMessage
						error
					}{nullMsg, err}
					return err

				}
			}
		}
		streamView.Editable = true
		streamView.Autoscroll = true
		streamView.Title = "Stream"
		streamView.Write([]byte(initialMessage.Stream))

		topicView.Editable = true
		topicView.Autoscroll = true
		topicView.Title = "Topic"
		topicView.Write([]byte(initialMessage.Topic))

		contentView.Editable = true
		contentView.Autoscroll = true
		contentView.Wrap = true
		contentView.Title = "Message"
		contentView.Write([]byte(initialMessage.Content))

		if err := g.SetCurrentView("stream-view-content"); err != nil {
			streamMessageSendResultChannel <- struct {
				zulip.OutgoingStreamMessage
				error
			}{nullMsg, err}
			return err
		}
		return nil
	})

	return streamMessageSendResultChannel
}

// TODO: private message version of showNewStreamMessagePrompt

func showHelp(g *gocui.Gui, v *gocui.View) error {
	parseCmdLine("help")
	return nil
}

func cuiQuit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
