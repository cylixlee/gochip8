package main

import (
	"errors"
	"fmt"
	"gochip8/internal"
	"os"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	SCALE           = 15
	WINDOW_WIDTH    = internal.SCREEN_WIDTH * SCALE
	WINDOW_HEIGHT   = internal.SCREEN_HEIGHT * SCALE
	TICKS_PER_FRAME = 10

	FRAMERATE_LIMIT        = 60
	MILLISECONDS_PER_FRAME = 1000 / FRAMERATE_LIMIT
)

func main() {
	if err := emulatorMain(); err != nil {
		sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_ERROR, "Error", err.Error(), nil)
	}
}

func emulatorMain() (out error) {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: chip8-emulator path/to/game")
	}
	emulator := internal.NewEmulator()
	rom, err := os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}
	emulator.Load(rom)

	if err = sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return err
	}
	defer sdl.Quit()

	window, renderer, err := sdl.CreateWindowAndRenderer(WINDOW_WIDTH, WINDOW_HEIGHT, sdl.WINDOW_HIDDEN)
	if err != nil {
		return err
	}
	defer func() {
		if err := renderer.Destroy(); err != nil {
			out = err
			return
		}
		if err = window.Destroy(); err != nil {
			out = err
			return
		}
	}()

	window.SetTitle("CHIP8 Emulator Go")
	window.Show()

gameloop:
	for {
		start := sdl.GetTicks64()
		event := sdl.PollEvent()
		for event != nil {
			switch event.GetType() {
			case sdl.QUIT:
				break gameloop
			case sdl.KEYDOWN:
				keyevent, ok := event.(*sdl.KeyboardEvent)
				if !ok {
					return errors.New("invalid keyboard event")
				}
				button := key2Button(keyevent.Keysym.Sym)
				if button > 0 {
					emulator.Input(button, true)
				}
			case sdl.KEYUP:
				keyevent, ok := event.(*sdl.KeyboardEvent)
				if !ok {
					return errors.New("invalid keyboard event")
				}
				button := key2Button(keyevent.Keysym.Sym)
				if button > 0 {
					emulator.Input(button, false)
				}
			}
			event = sdl.PollEvent()
		}

		for range TICKS_PER_FRAME {
			emulator.Tick()
		}
		emulator.TickTimers()
		drawScreen(emulator, renderer)

		elapsed := sdl.GetTicks64() - start
		if MILLISECONDS_PER_FRAME > elapsed {
			sdl.Delay(uint32(MILLISECONDS_PER_FRAME - elapsed))
		}
	}
	return out
}

func drawScreen(emulator *internal.Emulator, renderer *sdl.Renderer) error {
	if err := renderer.SetDrawColor(0, 0, 0, 255); err != nil {
		return err
	}
	if err := renderer.Clear(); err != nil {
		return err
	}

	screenBuffer := emulator.ScreenBuffer()
	if err := renderer.SetDrawColor(255, 255, 255, 255); err != nil {
		return err
	}

	for i, pixel := range screenBuffer {
		if pixel {
			x := i % internal.SCREEN_WIDTH
			y := i / internal.SCREEN_WIDTH
			rect := sdl.Rect{
				X: int32(x) * SCALE,
				Y: int32(y) * SCALE,
				W: SCALE,
				H: SCALE,
			}
			if err := renderer.FillRect(&rect); err != nil {
				return err
			}
		}
	}
	renderer.Present()
	return nil
}

func key2Button(key sdl.Keycode) int {
	switch key {
	case sdl.K_1:
		return 0x1
	case sdl.K_2:
		return 0x2
	case sdl.K_3:
		return 0x3
	case sdl.K_4:
		return 0xC
	case sdl.K_q:
		return 0x4
	case sdl.K_w:
		return 0x5
	case sdl.K_e:
		return 0x6
	case sdl.K_r:
		return 0xD
	case sdl.K_a:
		return 0x7
	case sdl.K_s:
		return 0x8
	case sdl.K_d:
		return 0x9
	case sdl.K_f:
		return 0xE
	case sdl.K_z:
		return 0xA
	case sdl.K_x:
		return 0x0
	case sdl.K_c:
		return 0xB
	case sdl.K_v:
		return 0xF
	}
	return -1
}
