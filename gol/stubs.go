package gol



var Gameoflifestring = "GameOfLife.Process"



type Response struct {
	World [][]byte
}

type Request struct {
	World     [][]byte
	P         Params
	Ratio     int
	Turn      int
}



