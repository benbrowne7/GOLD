package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/gol"
)


func handleError(err error) {
	fmt.Println("errar")
	log.Fatal(err)
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

func calculateNextState(sy, ey, h int, w int, world [][]byte, turn int) [][]byte {
	neww := make([][]byte, h)
	for i := range neww {
		neww[i] = make([]byte, w)
		copy(neww[i], world[i][:])
	}
	for i:=sy; i<ey; i++ {
		for z:=0; z<w; z++ {
			alive := checkSurrounding(i,z,h,world)
			if world[i][z] == 0 && alive==3 {
				neww[i][z] = 255
			} else {
				if world[i][z] == 255 && (alive<2 || alive>3) {
					neww[i][z] = 0
				}
			}
		}
	}
	return neww
}


func gameOfLife(sy, ey int, initialWorld [][]byte, p gol.Params, turn int) [][]byte {
	world := initialWorld
	world = calculateNextState(sy, ey, p.ImageHeight, p.ImageWidth, world, turn)
	return world
}
//processes GOL logic for slice of the 'world'
func worker(startY, endY int, initial [][]byte, iteration chan<- [][]byte, p gol.Params, turn int) {
	theMatrix := gameOfLife(startY,endY, initial, p, turn)
	iteration <- theMatrix[startY:endY][0:]
}

//starts the required number of worker threads and splits up the 'world'
func controller(ratio int, p gol.Params, world [][]byte, turn int) [][]byte {
	start := 0
	end := ratio
	temp := make(chan [][]byte)
	iteration := make([]chan [][]byte, p.Threads)
	for i:=0; i<p.Threads; i++ {
		iteration[i] = make(chan [][]byte)
	}
	fmt.Println("iteration maker started")
	go iterationMaker(iteration, temp, p)
	if p.Threads == 1 {
		go worker(0,p.ImageHeight,world,iteration[0], p, turn)
	} else {
		for i:=1; i<=p.Threads; i++ {
			go worker(start,end,world,iteration[i-1],p, turn)
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
func iterationMaker(iteration []chan [][]byte, temp chan [][]byte, p gol.Params) {
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


func ProcessGol(ratio int, p gol.Params, world [][]byte, turn int) [][]byte {
	x := controller(ratio, p, world, turn)
	return x

}
type GameOfLife struct {}

func (s *GameOfLife) Process(req gol.Request, res *gol.Response) (err error) {
	fmt.Println("in GOL")
	res.World = ProcessGol(req.Ratio, req.P, req.World, req.Turn)
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&GameOfLife{})
	fmt.Println("listening for clients")
	listener, err := net.Listen("tcp", ":" + *pAddr)
	if err != nil {
		fmt.Println("accept error")
		handleError(err)
	}
	defer listener.Close()
	rpc.Accept(listener)
}