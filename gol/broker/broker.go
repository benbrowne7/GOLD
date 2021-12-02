package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/gol"
)

//global variables which are updated every turn
var world [][]byte
var turn int

var pause int

//var terminate chan bool

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}

}

type Broker struct {
	client *rpc.Client
	listener net.Listener
}


//Pause method called by distributor when 'p' pressed
func (b *Broker) Pause(req gol.Empty, res *gol.Empty) (err error) {
	pause = 1
	return err
}
//Continue method called by distributor when 'p' pressed again
func (b *Broker) Continue(req gol.Empty, res *gol.Empty) (err error) {
	pause = 0
	return err
}

//broken method for shutting down first Server then Broker itself
func (b *Broker) Down(req gol.Request, res *gol.Update) (err error) {
	res.World = world
	res.Turn = turn
	errr := b.client.Call(gol.Gameoflifedown, gol.Empty{}, gol.Empty{})
	handleError(errr)
	//terminate <- true
	return err
}

//Method called by distributor which sends back the updated 'world' and corresponding 'turn' number - NOT called for every turn only when new data is required
func (b *Broker) Update(req gol.Empty, res *gol.Update) (err error) {
	res.World = world
	res.Turn = turn
	return err
}

//Main Broker Method called once by distributor: establishes connection with the server and calls the Server GOL logic for every turn
func (b *Broker) Broka(req gol.Request, res *gol.Final) (err error) {
	//serverAddr := flag.String("broker", "127.0.0.1:8050", "Address of broker instance")
	serverAddr := "127.0.0.1:8050"
	fmt.Println("Server: ", serverAddr)
	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("accept error")
		handleError(err)
	}
	defer client.Close()
	b.client = client

	turn = 0
	world = req.World

	for i:=0; i<req.P.Turns; i++ {
		if pause == 1 {
			//fmt.Println("broker paused")
			time.Sleep(10 * time.Millisecond)
			for {
				if pause == 0 {
					//fmt.Println("unpaused")
					time.Sleep(10 * time.Millisecond)
					break
				} else {
					continue
				}
			}
		}
		err = client.Call(gol.Gameoflifestring, req, res)
		if err != nil {
			handleError(err)
		}
		//updates the request so can be called with new values
		req.World = res.World
		req.Turn = req.Turn + 1
		world = res.World
		turn++
	}
	return err
}


func main() {

	//connects to controller as a server
	controllerAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&Broker{})
	var wg sync.WaitGroup
	fmt.Println("listening for clients")
	listener, err := net.Listen("tcp", ":" + *controllerAddr)
	if err != nil {
		fmt.Println("error connecting to controller")
		handleError(err)
	}
	pause = 0
	b := new(Broker)
	b.listener = listener

	wg.Add(1)
	go func() {
		defer wg.Done()
		rpc.Accept(listener)
	}()
	//<- terminate
	//time.Sleep(500 * time.Millisecond)
	wg.Wait()



}
