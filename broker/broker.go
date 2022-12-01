package main

import (
	"GOLServer/stubs"
	"GOLServer/util"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
)

var turn = 0
var turns int
var m sync.Mutex
var width int
var height int
var working = false
var aliveCells []util.Cell
var engines = make(map[int]*rpc.Client)

type GolEngine struct{}

func startEngine(client *rpc.Client, threads, id, engineHeight int, out chan<- []util.Cell, world [][]byte) {
	/*
		engineSlice := make([][]byte, engineHeight)
		for y := 0; y < engineHeight; y++ {
			engineSlice[y] = make([]byte, width)
			for x := range engineSlice[y] {
				engineSlice[y][x] = world[(id*engineHeight)+y][x]
			}
		}
	*/
	engineHalo := make([][]byte, 2)
	if id == 0 {
		engineHalo[0] = world[height-1]
	} else {
		engineHalo[0] = world[(engineHeight*id)-1]
	}
	if (id*engineHeight)+engineHeight == height {
		engineHalo[1] = world[0]
	} else {
		engineHalo[1] = world[(id*engineHeight)+engineHeight+1]
	}

	/*
		for y := 0; y < 2; y++ {
			engineHalo[y] = make([]byte, width)
			for j := range engineHalo[y] {
				if (id * engineHeight) == 0 {
					engineHalo[y][j] = world[height-1][j]
				} else {
					engineHalo[y][j] = world[(id*engineHeight)-1][j]
				}

				if (id*engineHeight)+engineHeight == height {
					engineHalo[y][j] = world[0][j]
				} else {
					engineHalo[y][j] = world[(id*engineHeight)+engineHeight+1][j]
				}
			}
		}
	*/

	args := stubs.EngineArgs{EngineSlice: world[id*engineHeight : (id*engineHeight)+engineHeight], EngineHalo: engineHalo, TWidth: width, THeight: height, EngineHeight: engineHeight, EngineID: id, Threads: threads}
	response := new(stubs.EngineResponse)

	err := client.Call(stubs.ProcessTurn, args, response)
	if err != nil {
		log.Fatal("Error when starting engine with ID: "+strconv.Itoa(id), err)
	}
	out <- response.AliveCells
}

func emptyWorld(w, h int) [][]byte {
	world := make([][]uint8, height)
	for i := 0; i < height; i++ {
		world[i] = make([]uint8, width)
		for j := range world[i] {
			world[i][j] = 0
		}
	}

	return world
}

func calculateAliveCells(width, height int, world [][]byte) []util.Cell {
	var newCell []util.Cell
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			if world[x][y] == 0xff {
				newCell = append(newCell, util.Cell{X: y, Y: x})
			}
		}
	}
	return newCell
}

func (g *GolEngine) ProcessTurns(args stubs.GolArgs, res *stubs.GolAliveCells) (err error) {
	turns = args.Turns
	turn = 0
	world := args.World
	width = args.Width
	height = args.Height
	working = true
	if turns == 0 {
		aliveCells = calculateAliveCells(width, height, world)
	}

	fmt.Printf("======= Job Recieved ========\nDimensions: %dx%d\nEngines: %d\nEngine Threads: %d\nTurns: %d\n", args.Width, args.Height, args.Engines, args.Threads, args.Turns)
	//aliveCells = calculateAliveCells(width, height, world) // initialise with current alive for 0 turn tests

	engineCount := len(engines)

	if args.Engines > engineCount {
		log.Fatal("Controller requested more Engines than we have connected...")
	} else if args.Engines == 0 {
		engineCount = len(engines)
	} else {
		engineCount = args.Engines
	}

	engineHeight := height / engineCount
	fmt.Printf("\nUsing %d Engines, with an Engine Height of %d", engineCount, engineHeight)

	out := make([]chan []util.Cell, engineCount)
	for i := range out {
		out[i] = make(chan []util.Cell)
	}

	for turn < turns {
		m.Lock()

		fmt.Printf("\n=== Began Turn %d ===\n", turn)

		for i := 0; i < engineCount; i++ {
			fmt.Printf("Starting Engine (%d) between Y: %d and Y: %d", i, engineHeight*i, engineHeight*i+engineHeight)
			go startEngine(engines[i], args.Threads, i, engineHeight, out[i], world)
		}

		nextWorld := emptyWorld(width, height)
		aliveCells = nil

		for i := 0; i < engineCount; i++ {

			var engineCells = <-out[i]
			aliveCells = append(aliveCells, engineCells...)

			fmt.Printf("\nEngine (%d) returned %d Alive Cells...", i, len(engineCells))

			for _, cell := range engineCells {
				nextWorld[cell.Y][cell.X] = 255
			}
		}

		fmt.Printf("\n- Finished Turn %d ! -\nEngines: %d\nAlive Cells: %d", turn, engineCount, len(aliveCells))
		world = nextWorld

		turn++
		m.Unlock()
	}

	fmt.Println("\n================== JOB FINISHED ===========================\n")
	fmt.Println("Returning " + strconv.Itoa(len(aliveCells)) + " to local controller")
	fmt.Println("\n================== JOB FINISHED ===========================\n\n\n")
	res.TurnsComplete = turns
	res.AliveCells = aliveCells
	working = false

	return
}

func (g *GolEngine) DoTick(_ bool, res *stubs.TickReport) (err error) {
	m.Lock()
	if working {
		res.AliveCount = len(aliveCells)
		res.Turns = turn
	} else {
		res.AliveCount = 0
		res.Turns = 0
	}
	m.Unlock()
	return
}

func (g *GolEngine) PauseEngine(_ bool, res *stubs.EngineStatus) (err error) {
	m.Lock()
	fmt.Println("Pausing Engines on turn: " + strconv.Itoa(turn))
	res.Turn = turn
	res.Working = working
	return
}

func (g *GolEngine) ResumeEngine(_ bool, res *stubs.EngineStatus) (err error) {
	fmt.Println("Resuming Engines from turn: " + strconv.Itoa(turn))
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
	fmt.Println("Starting shutdown process...")
	for id := range engines {
		fmt.Println("Shutting down Engine with ID: " + strconv.Itoa(id))
		engines[id].Call(stubs.KillEngine, true, true)
	}
	fmt.Println("Shutting down Broker...")
	os.Exit(0)
	return
}

func connectEngines() {
	content, err := ioutil.ReadFile("engines.txt")
	if err != nil {
		log.Fatal("Failed to read engines.txt file, can't really do much without any engines")
	}
	ips := strings.Split(string(content), "\n")
	//var ips = []string{"54.166.236.125:8031"}
	for id, ip := range ips {
		fmt.Println("\nConnecting to Engine with IP: " + ip)
		engine, e := rpc.Dial("tcp", ip)
		if e != nil {
			fmt.Println("Engine Connection FAILED:", e)
		} else {
			engines[id] = engine
			fmt.Println("Connected...")
		}
	}
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	fmt.Println("Game Of Life Broker V2 (takes threads and engines) listening on port: " + *pAddr)

	connectEngines()
	fmt.Println("\nConnected to " + strconv.Itoa(len(engines)) + " GOL Engines.")

	rpc.Register(&GolEngine{})

	listener, _ := net.Listen("tcp", ":"+*pAddr)

	defer listener.Close()
	rpc.Accept(listener)
}
