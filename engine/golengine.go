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

func calculateNextState(width, height int, world [][]byte) [][]byte {
	newWorld := make([][]byte, width)
	for i := range newWorld {
		newWorld[i] = make([]byte, height)
	}
	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {
			neighbours := getLiveNeighbours(width, height, world, i, j)
			if world[i][j] == 0xff && (neighbours < 2 || neighbours > 3) {
				newWorld[i][j] = 0x0
			} else if world[i][j] == 0x0 && neighbours == 3 {
				newWorld[i][j] = 0xff
			} else {
				newWorld[i][j] = world[i][j]
			}
		}
	}
	return newWorld
}

func calculateAliveCells(width, height int, world [][]byte) []util.Cell {
	newCell := []util.Cell{}
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			if world[x][y] == 0xff {
				newCell = append(newCell, util.Cell{y, x})
			}
		}
	}
	return newCell
}

func calculateAliveCount(world [][]byte) int {
	count := 0
	for x := range world {
		for y := range world[x] {
			if isAlive(world[x][y]) {
				count++
			}
		}
	}
	return count
}

func (g *GolEngine) ProcessTurn(args stubs.EngineArgs, res *stubs.EngineResponse) (err error) {
	m.Lock()
	world = args.TotalWorld

	fmt.Println("Engine Processing Turn between Y: " + strconv.Itoa(args.Offset) + " and Y: " + strconv.Itoa(args.Offset+args.Height))
	fmt.Println("Total world given has " + strconv.Itoa(calculateAliveCount(world)) + " alive cells...")

	aliveCells := []util.Cell{}
	for i := 0; i < args.TWidth; i++ {
		for j := args.Offset; j < args.Offset+args.Height; j++ {
			neighbours := getLiveNeighbours(args.TWidth, args.THeight, world, i, j)
			if world[i][j] == 0xff && (neighbours < 2 || neighbours > 3) {
				// cell dies, don't add to alive cells (duh)
			} else if world[i][j] == 0x0 && neighbours == 3 {
				aliveCells = append(aliveCells, util.Cell{X: j, Y: i})
			} else {
				if isAlive(world[i][j]) {
					aliveCells = append(aliveCells, util.Cell{X: j, Y: i})
				}
			}
		}
	}

	res.AliveCells = aliveCells
	m.Unlock()
	return
}

func (g *GolEngine) ProcessTurns(args stubs.GolArgs, res *stubs.GolAliveCells) (err error) {
	if !working { // If ProcessTurns is called again, it's a new client connection, continue working on current job
		turns = args.Turns
		turn = 0
		world = args.World
		width = args.Width
		height = args.Height
		working = true

		n := 0
		for n < 10 {
			n++
			fmt.Println("========== STARTING PROCESSING " + strconv.Itoa(turn) + "/" + strconv.Itoa(args.Turns) + "TURNS ==========")
		}

	} else {
		fmt.Println("Client called ProcessTurns while still working, continuing work")
	}

	for turn < turns {
		m.Lock()
		if turn%50 == 0 {
			fmt.Println("Engine Processing Turn: " + strconv.Itoa(turn))
		}
		world = calculateNextState(width, height, world)
		turn++
		m.Unlock()
	}

	res.TurnsComplete = turns
	res.AliveCells = calculateAliveCells(width, height, world)
	working = false
	n := 0
	for n < 10 {
		n++
		fmt.Println("========== FINISHED PROCESSING ALL " + strconv.Itoa(turn) + " TURNS ==========")
	}
	return
}

func (g *GolEngine) DoTick(_ bool, res *stubs.TickReport) (err error) {
	fmt.Println("Got do tick request...")
	m.Lock()
	res.AliveCount = calculateAliveCount(world)
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
	res.AliveCells = calculateAliveCells(width, height, world)
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
	fmt.Println("Super Cool Distributed Game of Life Engine is running on port: " + *pAddr)

	rpc.Register(&GolEngine{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
