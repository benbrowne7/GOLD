package gol



var Brokerstring = "Broker.Broka"
var Gameoflifestring = "GameOfLife.Process"
var Brokerupdate = "Broker.Update"
var Brokerdown = "Broker.Down"
var Brokerpause = "Broker.Pause"
var Brokercontinue = "Broker.Continue"
var Gameoflifedown = "GameOfLife.Down"



type Final struct {
	World [][]byte
}

type Request struct {
	World     [][]byte
	P         Params
	Ratio     int
	Turn      int
}

type Update struct {
	World [][]byte
	Turn int
}

type Empty struct {
}

type Pause struct {
	X int
}



