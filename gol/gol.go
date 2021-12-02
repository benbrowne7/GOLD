package gol

import (
	"fmt"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {


	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	file := make(chan string)
	out := make(chan uint8)
	in := make(chan uint8)

	fmt.Println("channels made")

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: file,
		output:   out,
		input:    in,
	}
	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: file,
		ioOutput:   out,
		ioInput:    in,
	}
	distributor(p, distributorChannels, keyPresses)

}
