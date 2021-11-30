package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"strconv"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

//var GameOfLife = "GameOfLife.Process"


type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}


func handleError(err error) {
	log.Fatal(err)
}


//counts number of live cells given a world
func nAlive(p Params, world [][]byte) int {
	c := 0
	g := world
	for i:= 0; i<p.ImageHeight; i++ {
		for z:= 0; z<p.ImageWidth; z++ {
			if g[i][z] == 255 {
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
	fileout := filename + "x" + strconv.Itoa(turn) + "-" + strconv.Itoa(p.Threads)
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
func keyPresses(k <-chan rune, world [][]byte, c distributorChannels, p Params, filename string, turn *int, pause, resume chan bool, in chan [][]byte) {
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
				sendState(2, x, c)

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


func distributor(p Params, c distributorChannels, k <-chan rune) {
	//brokerAddr := flag.String("broker","127.0.0.1:8030","IP:port string to connect to as server")
	//flag.Parse()
	brokerAddr := "127.0.0.1:8030"
	fmt.Println("Server: ", brokerAddr)
	client, err := rpc.Dial("tcp", brokerAddr)
	if err != nil {
		fmt.Println("error connecting to broker")
		handleError(err)
	}
	defer client.Close()


	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	turn := 0

	//makes 2d slice to store world
	inital := make([][]byte, p.ImageHeight)
	for i := range inital {
		inital[i] = make([]byte, p.ImageWidth)
	}
	//make channels
	in := make(chan [][]byte)
	pause := make(chan bool)
	resume := make(chan bool)
	go keyPresses(k, inital, c, p, filename, &turn, pause, resume, in)

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


	//used for correctly spilting up the world
	ratio := p.ImageHeight/p.Threads

	//channels and goroutines for AliveCellsCount (every 2 sec)
	nalive := make(chan int, p.Threads)
	everytwo := make(chan bool)
	count := make(chan int)
	finished := false
	go ticka(everytwo, nalive, count)
	go aliveSender(count, &turn, c)
	fmt.Println("aliveSender+ticker routines started")

	//logic to control whether a turn is executed, execution paused or AliveCellCount funcs

	world := inital
	req := Request{
		World:     world,
		P:         p,
		Ratio:     ratio,
		Turn:      0,
	}

	res := new(Response)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = client.Call(Brokerstring, req, res)
		finished = true
		if err != nil {
			handleError(err)
		}
	}()

	for {
		if finished == true {
			world = res.World
			break
		}
		select {
		case command := <- everytwo:
			switch command {
			case true:
				ress := new(Alive)
				reqq := Empty{}
				client.Call(Brokeralive, reqq, ress)
				world = ress.World
				turn = ress.Turn
				x := nAlive(p, world)
				nalive <- x
				fmt.Println("x:", x)
			}
		//case command := <- pause:
		//	switch command {
		//	case true:
		//		fmt.Println("Current turn: ", turn)
			//	<- resume
			//	fmt.Println("Continuing")
			//}
		//default:
			//world = res.World
			//go update(world, in)
			//c.events <- TurnComplete{CompletedTurns: turn}
		}
	}

	if p.Turns == 0{
		finalData = inital
	} else {
		finalData = world
	}

	fmt.Println("final data")

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