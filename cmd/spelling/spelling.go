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
	practice := flag.Bool("practice", false, "enable practice-only keys (history, examples) that skip typing")
	config := flag.String("config", "", "key binding file (default: keybindings.toml in the user config directory; created if missing)")
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

	keyPath := *config
	if keyPath == "" {
		p, err := game.DefaultKeyBindingsPath()
		if err != nil {
			log.Fatal(err)
		}
		keyPath = p
	}
	keys, err := game.LoadKeyBindings(keyPath)
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range keys.Missing() {
		log.Printf("%s: %s", keyPath, m)
	}

	newMatch := func() (*match.Match, error) {
		return match.New(elfs, func() bool { return rand.IntN(2) == 0 })
	}
	scene, err := game.NewMatchScene(newMatch, keys, *practice)
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowSize(game.WindowWidth, game.WindowHeight)
	ebiten.SetWindowTitle("Spelling")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
	ebiten.SetTPS(match.TicksPerSecond)

	if err := ebiten.RunGame(game.NewGame(scene)); err != nil {
		log.Fatal(err)
	}
}
