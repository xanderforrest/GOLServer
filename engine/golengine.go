package main

import (
	"GOLServer/stubs"
	"GOLServer/util"
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"
)

type GolEngine struct{}

var engineSlice [][]byte
var halo [][]byte
var engineHeight int
var turn = 0
var TWidth int
var m sync.Mutex
var working = false
var aliveCells = []util.Cell{}

func isAlive(cell byte) bool {
	if cell == 255 {
		return true
	}
	return false
}

func getLiveNeighbours(x, y int) int {
	var alive = 0
	var widthLeft int
	var widthRight int
	var heightUp int
	var heightDown int

	if x == 0 {
		widthLeft = TWidth - 1
	} else {
		widthLeft = x - 1
	}
	if x == TWidth-1 {
		widthRight = 0
	} else {
		widthRight = x + 1
	}

	if y == 0 {
		if isAlive(halo[0][widthLeft]) {
			alive = alive + 1
		}
		if isAlive(halo[0][x]) {
			alive = alive + 1
		}
		if isAlive(halo[0][widthRight]) {
			alive = alive + 1
		}
	} else {
		heightUp = y - 1

		if isAlive(engineSlice[heightUp][widthLeft]) {
			alive = alive + 1
		}
		if isAlive(engineSlice[heightUp][x]) {
			alive = alive + 1
		}
		if isAlive(engineSlice[heightUp][widthRight]) {
			alive = alive + 1
		}
	}

	if y == engineHeight-1 {
		if isAlive(halo[1][widthLeft]) {
			alive = alive + 1
		}
		if isAlive(halo[1][x]) {
			alive = alive + 1
		}
		if isAlive(halo[1][widthRight]) {
			alive = alive + 1
		}
	} else {
		heightDown = y + 1

		if isAlive(engineSlice[heightDown][widthLeft]) {
			alive = alive + 1
		}
		if isAlive(engineSlice[heightDown][x]) {
			alive = alive + 1
		}
		if isAlive(engineSlice[heightDown][widthRight]) {
			alive = alive + 1
		}
	}

	if isAlive(engineSlice[y][widthLeft]) {
		alive = alive + 1
	}
	if isAlive(engineSlice[y][widthRight]) {
		alive = alive + 1
	}

	return alive
}

func worker(startY, endY, yOffset, TWidth, engineHeight int, out chan<- []util.Cell) {
	workersCells := []util.Cell{}

	if endY > engineHeight {
		endY = engineHeight
	}

	for y := startY; y < endY; y++ {
		for x := 0; x < TWidth; x++ {
			neighbours := getLiveNeighbours(x, y)
			if engineSlice[y][x] == 0xff && (neighbours < 2 || neighbours > 3) {
				// cell dies, don't add to alive cells (duh)
			} else if engineSlice[y][x] == 0x0 && neighbours == 3 {
				workersCells = append(workersCells, util.Cell{X: x, Y: y + yOffset})
			} else {
				if isAlive(engineSlice[y][x]) {
					workersCells = append(workersCells, util.Cell{X: x, Y: y + yOffset})
				}
			}
		}
	}
	out <- workersCells
}

func (g *GolEngine) ProcessTurn(args stubs.EngineArgs, res *stubs.EngineResponse) (err error) {
	m.Lock()
	working = true
	engineSlice = args.EngineSlice
	halo = args.EngineHalo
	engineHeight = args.EngineHeight
	TWidth = args.TWidth
	aliveCells = []util.Cell{}
	yOffset := args.EngineHeight * args.EngineID

	workerHeight := engineHeight / args.Threads
	if engineHeight%args.Threads > 0 {
		workerHeight++
	}

	out := make([]chan []util.Cell, args.Threads)
	for i := range out {
		fmt.Println("Creating Channel for Thread with ID " + strconv.Itoa(i))
		out[i] = make(chan []util.Cell)
	}

	for i := 0; i < args.Threads; i++ {
		//fmt.Println("Starting worker between Y: " + strconv.Itoa(i*workerHeight) + ", " + strconv.Itoa((i+1)*workerHeight))
		var startY = i * workerHeight
		fmt.Println(strconv.Itoa(i) + " - Worker Processing Turn between Y: " + strconv.Itoa(startY) + " and Y: " + strconv.Itoa(startY+workerHeight))
		go worker(startY, startY+workerHeight, yOffset, args.TWidth, engineHeight, out[i])
	}

	for i := 0; i < args.Threads; i++ {

		var returnedCells = <-out[i]
		fmt.Println("Processing " + strconv.Itoa(len(returnedCells)) + " Alive Cells from Worker ID: " + strconv.Itoa(i))
		if len(returnedCells) > 0 {
			fmt.Println("First Cell from Worker " + strconv.Itoa(i) + ": X" + strconv.Itoa(returnedCells[0].X) + " Y" + strconv.Itoa(returnedCells[0].Y))
		}
		aliveCells = append(aliveCells, returnedCells...)

	}

	fmt.Println("Retunring " + strconv.Itoa(len(aliveCells)) + " cells to the Controller...")
	res.AliveCells = aliveCells
	working = false
	m.Unlock()
	return
}

func (g *GolEngine) DoTick(_ bool, res *stubs.TickReport) (err error) {
	fmt.Println("Got do tick request...")
	m.Lock()
	res.AliveCount = len(aliveCells)
	res.Turns = turn
	m.Unlock()
	return
}

func (g *GolEngine) PauseEngine(_ bool, res *stubs.EngineStatus) (err error) {
	m.Lock()
	fmt.Println("pausing engine on turn " + strconv.Itoa(turn) + "...")
	res.Turn = turn
	res.Working = working
	return
}

func (g *GolEngine) ResumeEngine(_ bool, res *stubs.EngineStatus) (err error) {
	fmt.Println("resuming engine from turn " + strconv.Itoa(turn))
	res.Turn = turn
	res.Working = working
	m.Unlock()
	return
}

func (g *GolEngine) InterruptEngine(_ bool, res *stubs.GolAliveCells) (err error) {
	m.Lock()
	fmt.Println("Interrupt triggered, returning current work to controller.")

	res.TurnsComplete = turn
	res.AliveCells = aliveCells
	m.Unlock()
	return
}

func (g *GolEngine) CheckStatus(_ bool, res *stubs.EngineStatus) (err error) {
	m.Lock()
	res.Turn = turn
	res.Working = working
	m.Unlock()
	return
}

func (g *GolEngine) KillEngine(_ bool, _ *bool) (err error) {
	fmt.Println("Shutting down...")
	os.Exit(0)
	return
}

func main() {
	pAddr := flag.String("port", "8031", "Port to listen on")
	flag.Parse()
	fmt.Println("Super Cool Distributed Game of Life Engine V4 (DEBUG threaded + overflow check) is running on port: " + *pAddr)

	rpc.Register(&GolEngine{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
