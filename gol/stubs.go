package gol



var Brokerstring = "Broker.Broka"
var Gameoflifestring = "GameOfLife.Process"
var Brokeralive = "Broker.Alive"



type Response struct {
	World [][]byte
}

type Request struct {
	World     [][]byte
	P         Params
	Ratio     int
	Turn      int
}

type Alive struct {
	World [][]byte
	Turn int
}

type Empty struct {
}


