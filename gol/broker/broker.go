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


var world [][]byte
var turn int

func handleError(err error) {
log.Fatal(err)
}

type Broker struct {}

func (b *Broker) Alive(req gol.Empty, res *gol.Alive) (err error) {
	res.World = world
	res.Turn = turn
	return err
}


func (b *Broker) Broka(req gol.Request, res *gol.Response) (err error) {
	//serverAddr := flag.String("broker", "127.0.0.1:8050", "Address of broker instance")
	serverAddr := "127.0.0.1:8050"
	fmt.Println("Server: ", serverAddr)
	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("accept error")
		handleError(err)
	}
	defer client.Close()

	turn = 0
	world = req.World

	for i:=0; i<req.P.Turns; i++ {
		err = client.Call(gol.Gameoflifestring, req, res)
		if err != nil {
			handleError(err)
		}
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
	defer listener.Close()
	wg.Add(1)
	go func() {
		defer wg.Done()
		rpc.Accept(listener)
	}()
	wg.Wait()


}
