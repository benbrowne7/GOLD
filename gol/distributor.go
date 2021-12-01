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



type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

//handles any errors returned by RPC calls, prints error and exits
func handleError(err error) {
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}
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


//handles different keypresses and does required things
func keyPresses(k <-chan rune, world [][]byte, c distributorChannels, p Params, filename string, turn *int, pause, resume chan bool, in chan [][]byte, client *rpc.Client) {
	fmt.Println("in keypresses")
	for {
		select {
		//calls broker to get update for turn number + world then outputs to pgm
		case command := <- k:
			switch command {
			case 's':
				ress := new(Update)
				client.Call(Brokerupdate, Empty{}, ress)
				x := ress.Turn
				sendWorld(ress.World, c, p, filename, ress.Turn)
				fileout := filename + "x" + strconv.Itoa(x) + "-" + strconv.Itoa(p.Threads)
				imageOut(x, fileout, c)
				sendState(1, x, c)
			}
			//closes the local controller, but broker/server still running
			switch command {
			case 'q':
				ress := new(Update)
				client.Call(Brokerupdate, Empty{}, ress)
				x := ress.Turn
				sendState(2, x, c)
				client.Close()
			}
			//broken but supposedly calls broker which calls server and shuts all down properly
			switch command {
			case 'k':
				ress := new(Update)
				err := client.Call(Brokerdown, Empty{}, ress)
				handleError(err)
				x := ress.Turn
				sendWorld(ress.World, c, p, filename, ress.Turn)
				fileout := filename + "x" + strconv.Itoa(x) + "-" + strconv.Itoa(p.Threads)
				imageOut(x, fileout, c)
				sendState(2, x, c)
				client.Close()
			}
			//updates turn number, calls broker to pause, waits for another 'p' keypress then calls broker to continue
			switch command {
			case 'p':
				ress := new(Update)
				client.Call(Brokerupdate, Empty{}, ress)
				fmt.Println("Paused turn:", ress.Turn)
				p1 := new(Empty)
				client.Call(Brokerpause, Empty{}, p1)
				pause <- true
				sendState(0, ress.Turn, c)
				for {
					test := <- k
					if test == 'p' {
						p2 := new(Empty)
						client.Call(Brokercontinue, Empty{}, p2)
						resume <- true
						break
					}
				}
			}
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
	go keyPresses(k, inital, c, p, filename, &turn, pause, resume, in, client)

	//sends cellFlipped events for every live cell in inital world
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			bytez := <- c.ioInput
			inital[y][x] = bytez
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

	//creates inital request with all needed info
	world := inital
	req := Request{
		World:     world,
		P:         p,
		Ratio:     ratio,
		Turn:      0,
	}

	//calls the main broker method - Broker.Broka in a go func
	res := new(Final)
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

	//for loop which handles the live cells every two seconds count, pauses if 'p' pressed and breaks when Broker.Broka finishes
	for {
		if finished == true {
			world = res.World
			break
		}
		select {
		case command := <- everytwo:
			switch command {
			case true:
				ress := new(Update)
				reqq := Empty{}
				client.Call(Brokerupdate, reqq, ress)
				world = ress.World
				turn = ress.Turn
				x := nAlive(p, world)
				nalive <- x
				fmt.Println("x:", x)
			}
		case command := <- pause:
			switch command {
			case true:
				<- resume
			}
		}
	}

	//sets finalData up properly
	if p.Turns == 0{
		finalData = inital
	} else {
		finalData = world
	}

	fmt.Println("final data")

	//writing world to .pgm file
	sendWorld(finalData, c, p, filename, p.Turns)


	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	fileout := filename + "x" + strconv.Itoa(p.Turns) + "-" + strconv.Itoa(p.Threads)

	imageOut(p.Turns, fileout, c)

	alive := calculateAliveCells(p, finalData)
	fmt.Println("turns executed")


	//Reports the final state using FinalTurnCompleteEvent.
	final := FinalTurnComplete{
		CompletedTurns: p.Turns,
		Alive:          alive,
	}
	c.events <- final
	fmt.Println("final state sent")

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}