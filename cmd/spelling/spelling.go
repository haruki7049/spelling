package main

import (
	"flag"
	"log"
	"math/rand/v2"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/haruki7049/spelling/internal/game"
	"github.com/haruki7049/spelling/internal/match"
)

func main() {
	playerELF := flag.String("elf", "", "RISC-V ELF to load as your language implementation (default: none, type raw Idea)")
	opponentELF := flag.String("opponent-elf", "", "RISC-V ELF to run as the CPU opponent (default: an idle opponent)")
	flag.Parse()

	var elfs [2][]byte
	for i, path := range []string{*playerELF, *opponentELF} {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		elfs[i] = data
	}

	m, err := match.New(elfs, func() bool { return rand.IntN(2) == 0 })
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowSize(game.WindowWidth, game.WindowHeight)
	ebiten.SetWindowTitle("Spelling")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
	ebiten.SetTPS(match.TicksPerSecond)

	if err := ebiten.RunGame(game.NewGame(game.NewMatchScene(m))); err != nil {
		log.Fatal(err)
	}
}
