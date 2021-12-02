package gol

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

var wg sync.WaitGroup

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

//counts number of live cells given a world
func nAlive(p Params, world [][]byte) int {
	c := 0
	for i:= 0; i<p.ImageHeight; i++ {
		for z:= 0; z<p.ImageWidth; z++ {
			if world[i][z] == 255 {
				c++
			}
		}
	}
	return c
}

//builds the array of live cells for testing
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	alive := make([]util.Cell,0)

	for i:=0; i<p.ImageHeight; i++ {
		for z:=0; z<p.ImageWidth; z++ {
			if world[i][z]==255 {
				var x util.Cell
				x.X = z
				x.Y = i
				alive = append(alive, x)
			}
		}
	}
	return alive
}

//sends a signal to distributor every 2 sec
func ticka(everytwo chan bool, nalive chan int, count chan int) {
	ticker := time.NewTicker(2 * time.Second)
	for _ = range ticker.C {
		everytwo <- true
		go bufferget(nalive, count)
	}
}
//gets number of live cells and sends to count chan
func bufferget(nalive chan int, count chan int) {
	x := <- nalive
	count <- x
	fmt.Println("sent value to count:", x)

}
//sends an aliveEvent to c.events
func aliveSender(count chan int, turn *int, c distributorChannels) {
	for {
		fmt.Println("aliveSender waiting...")
		x := <- count
		aliveEvent := AliveCellsCount{
			CompletedTurns: *turn,
			CellsCount:     x,
		}
		c.events <- aliveEvent
	}
}

//functions for GOL logic
func mod(a, b int) int {
	return (a % b + b) % b
}
func checkSurrounding(i, z, dimension int, neww [][]byte) int {
	x := 0
	if neww[mod(i-1,dimension)][z] == 255 {x++}
	if neww[mod(i+1,dimension)][z] == 255 {x++}
	if neww[i][mod(z+1,dimension)] == 255 {x++}
	if neww[i][mod(z-1,dimension)] == 255 {x++}
	if neww[mod(i-1,dimension)][mod(z+1,dimension)] == 255 {x++}
	if neww[mod(i-1,dimension)][mod(z-1,dimension)] == 255 {x++}
	if neww[mod(i+1,dimension)][mod(z+1,dimension)] == 255 {x++}
	if neww[mod(i+1,dimension)][mod(z-1,dimension)] == 255 {x++}
	return x
}

func calculateNextState(sy, ey, h int, w int, world [][]byte, turn int, c distributorChannels) [][]byte {
	neww := make([][]byte, h)
	for i := range neww {
		neww[i] = make([]byte, w)
		copy(neww[i], world[i][:])
	}
	celly := CellFlipped{
		CompletedTurns: turn,
		Cell:           util.Cell{X: 0, Y: 0},
	}
	for i:=sy; i<ey; i++ {
		for z:=0; z<w; z++ {
			alive := checkSurrounding(i,z,h,world)
			if world[i][z] == 0 && alive==3 {
				neww[i][z] = 255
				celly.Cell.X = z
				celly.Cell.Y = i
				c.events <- celly
			} else {
				if world[i][z] == 255 && (alive<2 || alive>3) {
					neww[i][z] = 0
					celly.Cell.X = z
					celly.Cell.Y = i
					c.events <- celly
				}
			}
		}
	}
	return neww
}


func gameOfLife(sy, ey int, initialWorld [][]byte, p Params, turn int, c distributorChannels) [][]byte {
	world := initialWorld
	world = calculateNextState(sy, ey, p.ImageHeight, p.ImageWidth, world, turn, c)
	return world
}
//processes GOL logic for slice of the 'world'
func worker(startY, endY int, initial [][]byte, iteration chan<- [][]byte, p Params, turn int, c distributorChannels) {
	theMatrix := gameOfLife(startY,endY, initial, p, turn, c)
	iteration <- theMatrix[startY:endY][0:]
}

//starts the required number of worker threads and splits up the 'world'
func controller(ratio int, p Params, iteration []chan [][]byte, world [][]byte, turn int, c distributorChannels) [][]byte {
	start := 0
	end := ratio
	temp := make(chan [][]byte)
	go iterationMaker(iteration, temp, p)
	if p.Threads == 1 {
		go worker(0,p.ImageHeight,world,iteration[0], p, turn, c)
	} else {
		for i:=1; i<=p.Threads; i++ {
			go worker(start,end,world,iteration[i-1],p, turn, c)
			start = start + ratio
			if i==p.Threads-1 {
				end = p.ImageHeight
			} else {
				end = end + ratio
			}
		}
	}
	x := <- temp
	return x
}
//receives each slice from the workers and puts them back together, sends down temp chan
func iterationMaker(iteration []chan [][]byte, temp chan [][]byte, p Params) {
	if p.Threads==1 {
		g := <- iteration[0]
		temp <- g
	} else {
		var world [][]byte
		for x := range iteration {
			y := <- iteration[x]
			world = append(world, y...)
		}
		temp <- world
	}
}

//func for making/sending different states
func sendState(i, turn int, c distributorChannels) {
	state := StateChange{
		CompletedTurns: turn,
		NewState:       Executing,
	}
	if i==0 {
		state.NewState = Paused
	}
	if i==1 {
		state.NewState = Executing
	}
	if i==2 {
		state.NewState = Quitting
	}
	c.events <- state
}

//func for sending imageout events
func imageOut(turn int, fileout string, c distributorChannels) {
	imageevent := ImageOutputComplete{
		CompletedTurns: turn,
		Filename:       fileout,
	}
	c.events <- imageevent
}
//sends 'world' data byte by byte down c.ioOutput
func sendWorld(world [][]byte, c distributorChannels, p Params, filename string, turn int) {
	c.ioCommand <- ioOutput
	fileout := filename + "x" + strconv.Itoa(turn)
	c.ioFilename <- fileout
	for i:=0; i<p.ImageHeight; i++ {
		for z:=0; z<p.ImageWidth; z++{
			c.ioOutput <- world[i][z]
		}
	}
}
//used to update the 'world' for SDL Keypresses
func update(world [][]byte, in chan [][]byte) {
	in <- world
}
//handles different keypresses and does required things
func keyPresses(k <-chan rune, world [][]byte, in chan [][]byte, c distributorChannels, p Params, filename string, turn *int, pause, resume chan bool) {
	for {
		select {
		case command := <- k:
			switch command {
			case 's':
				x := *turn
				sendWorld(world, c, p, filename, x)
				fileout := filename + "x" + strconv.Itoa(x) + "-" + strconv.Itoa(p.Threads)
				imageOut(x, fileout, c)
				sendState(1, x, c)
			}
			switch command {
			case 'q':
				x := *turn
				sendWorld(world, c, p, filename, x)
				fileout := filename + "x" + strconv.Itoa(x) + "-" + strconv.Itoa(p.Threads)
				imageOut(x, fileout, c)
				sendState(2, x, c)
				os.Exit(0)
			}
			switch command {
			case 'p':
				pause <- true
				for {
					test := <- k
					if test == 'p' {
						resume <- true
						break
					}
				}
			}
		default:
			world = <- in
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, k <-chan rune) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	turn := 0
	fmt.Println(p.Turns)

	//makes 2d slice to store world
	inital := make([][]byte, p.ImageHeight)
	for i := range inital {
		inital[i] = make([]byte, p.ImageWidth)
	}
	//make channels
	in := make(chan [][]byte)
	pause := make(chan bool)
	resume := make(chan bool)

	go keyPresses(k, inital, in, c, p, filename, &turn, pause, resume)

	//sends cellFlipped events for every live cell in inital world
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			byte := <- c.ioInput
			inital[y][x] = byte
			if inital[y][x] == 255 {
				celly := CellFlipped{
					CompletedTurns: turn,
					Cell:           util.Cell{
						X: x,
						Y: y,
					},
				}
				c.events <- celly
			}
		}
	}

	fmt.Println("distributor initialised world")

	var finalData [][]uint8

	//create channels for each worker thread
	iteration := make([]chan [][]byte, p.Threads)
	for i:=0; i<p.Threads; i++ {
		iteration[i] = make(chan [][]byte)
	}

	//used for correctly spilting up the world
	ratio := p.ImageHeight/p.Threads

	//channels and goroutines for AliveCellsCount (every 2 sec)
	nalive := make(chan int, p.Threads)
	everytwo := make(chan bool)
	count := make(chan int)
	go ticka(everytwo, nalive, count)
	go aliveSender(count, &turn, c)
	fmt.Println("aliveSender+ticker routines started")

	//logic to control whether a turn is executed, execution paused or AliveCellCount funcs
	world := inital
	for i:=0; i<p.Turns; i++ {
		select {
		case command := <- everytwo:
			switch command {
			case true:
				x := nAlive(p, world)
				nalive <- x
				fmt.Println("x:", x)
			}
		case command := <- pause:
			switch command {
			case true:
				fmt.Println("Current turn: ", turn)
				<- resume
				fmt.Println("Continuing")
			}
		default:
			world = controller(ratio, p, iteration, world, turn, c)
			go update(world, in)
			c.events <- TurnComplete{CompletedTurns: turn}
			turn++
		}
	}
	finalData = world

	//writing world to .pgm file
	sendWorld(finalData, c, p, filename, turn)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	fileout := filename + "x" + strconv.Itoa(p.Turns) + "-" + strconv.Itoa(p.Threads)

	imageOut(turn, fileout, c)

	alive := calculateAliveCells(p, finalData)
	fmt.Println("turns executed")


	//Reports the final state using FinalTurnCompleteEvent.
	final := FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          alive,
	}
	c.events <- final
	fmt.Println("final state sent")

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}