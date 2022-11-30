package stubs

import "GOLServer/util"

var ProcessTurns = "GolEngine.ProcessTurns"
var DoTick = "GolEngine.DoTick"
var PauseEngine = "GolEngine.PauseEngine"
var ResumeEngine = "GolEngine.ResumeEngine"
var InterruptEngine = "GolEngine.InterruptEngine"
var CheckStatus = "GolEngine.CheckStatus"
var KillEngine = "GolEngine.KillEngine"
var ProcessTurn = "GolEngine.ProcessTurn"

type GolArgs struct {
	World                [][]byte
	Width, Height, Turns int
	Threads              int
	Engines              int
}

type EngineArgs struct {
	TotalWorld      [][]byte
	TWidth, THeight int
	Height          int
	Offset          int
	Threads         int
}

type EngineResponse struct {
	AliveCells []util.Cell
}

type GolAliveCells struct {
	TurnsComplete int
	AliveCells    []util.Cell
}

type TickReport struct {
	Turns      int
	AliveCount int
}

type EngineStatus struct {
	Working bool
	Turn    int
}
