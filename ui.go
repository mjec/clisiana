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
	var editor gocui.Editor = gocui.EditorFunc(cuiEditor)
	config.ui.Editor = editor

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
	case DebugMessage:
		if !DEBUG {
			return func(g *gocui.Gui) error { return nil }
		}
		str = fmt.Sprintf("DEBUG: %s\n", str)
	case ErrorMessage:
		str = fmt.Sprintf("ERROR: %s\n", str)
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

	g.SetCurrentView("cmd")
	return nil
}

func showHelp(g *gocui.Gui, v *gocui.View) error {
	parseLine("help")
	return nil
}

func cuiQuit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
