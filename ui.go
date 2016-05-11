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

	// Set up interface
	config.Interface = gocui.NewGui()
	if err := config.Interface.Init(); err != nil {
		log.Panicln(err)
	}
	defer config.Interface.Close()
	config.Interface.Cursor = true
	var editor gocui.Editor = gocui.EditorFunc(cuiEditor)
	config.Interface.Editor = editor

	config.Interface.SetLayout(layout)

	if err := setGlobalKeybindings(config.Interface); err != nil {
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
			switch msg.Type {
			case DebugMessage:
				if DEBUG {
					ifce.Execute(makeMainViewUpdater("*** DEBUG MESSAGE: " + msg.Message.Content))
				}
			default:
				ifce.Execute(makeMainViewUpdater(msg.Message.Content + "\n"))
			}
		}
	}(config.MainTextChannel, config.Interface)

	if err := config.Interface.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

	return nil
}

func makeMainViewUpdater(s string) gocui.Handler {
	return func(g *gocui.Gui) error {
		main, err := g.View("main")
		if err != nil {
			return err
		}
		fmt.Fprint(main, s)
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
