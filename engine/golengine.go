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

var world [][]byte
var turn = 0
var turns int
var m sync.Mutex
var width int
var height int
var working = false
var offset int
var eHeight int
var singleWorker = false
var listener net.Listener
var aliveCells = []util.Cell{}

func isAlive(cell byte) bool {
	if cell == 255 {
		return true
	}
	return false
}

func getLiveNeighbours(width, height int, world [][]byte, a, b int) int {
	var alive = 0
	var widthLeft int
	var widthRight int
	var heightUp int
	var heightDown int

	//fmt.Println("Getting neighbours\nWidth " + strconv.Itoa(width) + "\nHeight: " + strconv.Itoa(height) + "\na: " + strconv.Itoa(a) + "\nb: " + strconv.Itoa(b))

	if a == 0 {
		widthLeft = width - 1
	} else {
		widthLeft = a - 1
	}
	if a == width-1 {
		widthRight = 0
	} else {
		widthRight = a + 1
	}

	if b == 0 {
		heightUp = height - 1
	} else {
		heightUp = b - 1
	}

	if b == height-1 {
		heightDown = 0
	} else {
		heightDown = b + 1
	}

	if isAlive(world[widthLeft][b]) {
		alive = alive + 1
	}
	if isAlive(world[widthRight][b]) {
		alive = alive + 1
	}
	if isAlive(world[widthLeft][heightUp]) {
		alive = alive + 1
	}
	if isAlive(world[a][heightUp]) {
		alive = alive + 1
	}
	if isAlive(world[widthRight][heightUp]) {
		alive = alive + 1
	}
	if isAlive(world[widthLeft][heightDown]) {
		alive = alive + 1
	}
	if isAlive(world[a][heightDown]) {
		alive = alive + 1
	}
	if isAlive(world[widthRight][heightDown]) {
		alive = alive + 1
	}
	return alive
}

func worker(startY, endY, TWidth, THeight int, out chan<- []util.Cell) {
	workersCells := []util.Cell{}
	for i := 0; i < TWidth; i++ {
		for j := startY; j < endY; j++ {
			neighbours := getLiveNeighbours(TWidth, THeight, world, i, j)
			if world[i][j] == 0xff && (neighbours < 2 || neighbours > 3) {
				// cell dies, don't add to alive cells (duh)
			} else if world[i][j] == 0x0 && neighbours == 3 {
				workersCells = append(workersCells, util.Cell{X: j, Y: i})
			} else {
				if isAlive(world[i][j]) {
					workersCells = append(workersCells, util.Cell{X: j, Y: i})
				}
			}
		}
	}
	out <- workersCells
}

func (g *GolEngine) ProcessTurn(args stubs.EngineArgs, res *stubs.EngineResponse) (err error) {
	m.Lock()
	world = args.TotalWorld

	aliveCells = []util.Cell{}

	workerHeight := args.Height / args.Threads

	//fmt.Println("Starting image: " + filename + " with " + strconv.Itoa(p.Threads) + " threads and a worker height of: " + strconv.Itoa(workerHeight))

	out := make([]chan []util.Cell, args.Threads)
	for i := range out {
		fmt.Println("Creating Channel for Thread with ID " + strconv.Itoa(i))
		out[i] = make(chan []util.Cell)
	}

	for i := 0; i < args.Threads; i++ {
		//fmt.Println("Starting worker between Y: " + strconv.Itoa(i*workerHeight) + ", " + strconv.Itoa((i+1)*workerHeight))
		var startY = args.Offset + (i * workerHeight)
		fmt.Println(strconv.Itoa(i) + " - Worker Processing Turn between Y: " + strconv.Itoa(startY) + " and Y: " + strconv.Itoa(startY+workerHeight))
		go worker(startY, startY+workerHeight, args.TWidth, args.THeight, out[i])
	}

	for i := 0; i < args.Threads; i++ {

		var workerCells = <-out[i]
		aliveCells = append(aliveCells, workerCells...)

		fmt.Println("Processing " + strconv.Itoa(len(aliveCells)) + " Alive Cells from Worker ID: " + strconv.Itoa(i))
	}

	fmt.Println("Retunring " + strconv.Itoa(len(aliveCells)) + " cells to the Controller...")
	res.AliveCells = aliveCells
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
	fmt.Println("Super Cool Distributed Game of Life Engine V3 (threaded) is running on port: " + *pAddr)

	rpc.Register(&GolEngine{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
