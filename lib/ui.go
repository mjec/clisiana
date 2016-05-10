package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
)

// With thanks to https://github.com/martinlindhe/imgcat

// DisplayImage will show an octopus
func DisplayImage() error {
	// embed(imageAsPngBytes(i image.Image), w io.Writer)
	f, err := os.Open("octopus.png")
	m, _, _ := image.Decode(f)
	if err != nil {
		log.Println(":( I can't read the octopus file")
		log.Println(err)
	}
	b, _ := imageAsPngBytes(m)
	embed(b, os.Stdout)
	return nil
}

func embed(r io.Reader, w io.Writer) error {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return err
	}

	// tmux requires unrecognized OSC sequences to be wrapped with DCS tmux;
	// <sequence> ST, and for all ESCs in <sequence> to be replaced with ESC ESC. It
	// only accepts ESC backslash for ST.
	fmt.Fprint(w, "\033Ptmux;\033\033]1337;File=;inline=1:")

	encoder := base64.NewEncoder(base64.StdEncoding, w)
	_, err = encoder.Write(buf.Bytes())
	if err != nil {
		return err
	}
	encoder.Close()

	// More of the tmux workaround described above.
	fmt.Fprintln(w, "\a\033\\")
	return nil
}

func imageAsPngBytes(i image.Image) (io.Reader, error) {

	buf := new(bytes.Buffer)
	err := png.Encode(buf, i)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
