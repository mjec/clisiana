package main

import "github.com/jroimartin/gocui"

func setGlobalKeybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, cuiQuit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, cuiQuit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyF1, gocui.ModNone, showHelp); err != nil {
		return err
	}
	return nil
}

func setCmdViewKeybindings(g *gocui.Gui, viewName string) error {
	return nil
}

func cuiEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case ch != 0 && mod == 0:
		v.EditWrite(ch)
	case key == gocui.KeySpace:
		v.EditWrite(' ')
	case key == gocui.KeyCtrlH:
		fallthrough
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	case key == gocui.KeyDelete:
		v.EditDelete(false)
	case key == gocui.KeyInsert:
		v.Overwrite = !v.Overwrite
	case key == gocui.KeyCtrlL:
		clearCmdView()
	// case key == gocui.KeyEsc:
	// 	fmt.Printf("Esc: %d", len(v.Buffer()))
	// 	if len(v.Buffer()) > 0 {
	// 		clearCmdView()
	// 	} else {
	// 		// toggle view?
	// 	}
	// case key == gocui.KeyCtrlE:
	// 	v.MoveCursor(dx, dy, writeMode)
	// case key == gocui.KeyCtrlB:
	// 	// Back word
	// case key == gocui.KeyCtrlF:
	// 	// Forward a word
	// // case key ==
	case key == gocui.KeyCtrlD:
		fallthrough
	case key == gocui.KeyEnter:
		parseLine(v.Buffer())
		clearCmdView()
	// case key == gocui.KeyArrowDown:
	// 	v.MoveCursor(0, 1, false)
	// case key == gocui.KeyArrowUp:
	// 	v.MoveCursor(0, -1, false)
	case key == gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0, true)
	case key == gocui.KeyArrowRight:
		x, _ := v.Cursor()
		ox, _ := v.Origin()
		if x < len(v.Buffer())-ox-1 {
			v.MoveCursor(1, 0, true)
		}
	}

	// Up     -> up in history
	// Down   -> down in history
	// Tab    -> completion (double tab for list)
	// Ctrl-E -> End
	// Ctrl-A -> Start
	// Ctrl-B -> Back word
	// Ctrl-F -> Forward word
	// Ctrl-W -> Delete from start of line to cursor
	// Ctrl-K -> Delete from cursor to end of line
	// Ctrl-U -> Clear line
	// Ctrl-F -> Search history forward
	// Ctrl-R -> Search history backward
}

func clearCmdView() error {
	v, err := config.ui.View("cmd")
	if err != nil {
		return err
	}
	v.Clear()
	_, y := v.Origin()
	err = v.SetCursor(0, y)
	if err != nil {
		return err
	}
	err = v.SetOrigin(0, y)
	return err
}
