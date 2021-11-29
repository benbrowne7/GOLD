package gol



var GameOfLife = "GameOfLife.Pro"



type Response struct {
	World [][]byte
}

type Request struct {
	World     [][]byte
	P         Params
	Ratio     int
	Iteration []chan [][]byte
	Turn      int
}



