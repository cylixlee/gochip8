package internal

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/albenik/bcd"
)

const (
	RAM_SIZE       = 4 * 1024
	SCREEN_WIDTH   = 64
	SCREEN_HEIGHT  = 32
	REGISTER_COUNT = 16
	STACK_SIZE     = 16
	KEY_COUNT      = 16
	START_ADDRESS  = 0x200
)

type Emulator struct {
	programCount   uint16
	ram            [RAM_SIZE]byte
	screen         [SCREEN_WIDTH * SCREEN_HEIGHT]bool
	valueRegisters [REGISTER_COUNT]byte
	indexRegister  uint16
	stackPointer   uint16
	stack          [STACK_SIZE]uint16
	keys           [KEY_COUNT]bool
	delayTimer     byte
	soundTimer     byte
}

func NewEmulator() *Emulator {
	emulator := &Emulator{
		programCount: START_ADDRESS,
	}
	copy(emulator.ram[:], FontSet[:])
	return emulator
}

func (e Emulator) ScreenBuffer() []bool {
	return e.screen[:]
}

func (e *Emulator) Input(index int, pressed bool) {
	e.keys[index] = pressed
}

func (e *Emulator) Load(data []byte) {
	copy(e.ram[START_ADDRESS:START_ADDRESS+len(data)], data)
}

func (e *Emulator) Push(value uint16) {
	e.stack[e.stackPointer] = value
	e.stackPointer++
}

func (e *Emulator) Pop() uint16 {
	e.stackPointer--
	return e.stack[e.stackPointer]
}

func (e *Emulator) Tick() {
	opcode := e.Fetch()
	e.Execute(opcode)
}

func (e *Emulator) TickTimers() {
	if e.delayTimer > 0 {
		e.delayTimer--
	}

	if e.soundTimer > 0 {
		if e.soundTimer == 1 {
			fmt.Println("BEEP")
		}
		e.soundTimer--
	}
}

func (e *Emulator) Fetch() uint16 {
	higher := uint16(e.ram[e.programCount])
	lower := uint16(e.ram[e.programCount+1])
	opcode := (higher << 8) | lower
	e.programCount += 2
	return opcode
}

func (e *Emulator) Execute(opcode uint16) error {
	b1, b2, b3, b4 := (opcode&0xF000)>>12, (opcode&0x0F00)>>8, (opcode&0x00F0)>>4, opcode&0x000F
	switch b1 {
	case 0:
		// 0000, NOP
		if opcode == 0 {
			return nil
		} else if b3 == 0xE {
			// 00E0, CLS
			if b4 == 0 {
				e.screen = [SCREEN_WIDTH * SCREEN_HEIGHT]bool{}
			}
			// 00EE, RET
			if b4 == 0xE {
				returnAddress := e.Pop()
				e.programCount = returnAddress
			}
		} else {
			return unsupportedOpcode(opcode)
		}
	case 1: // 1NNN, JMP
		address := opcode & 0xFFF
		e.programCount = address
	case 2: // 2NNN, CALL
		address := opcode & 0xFFF
		e.Push(e.programCount)
		e.programCount = address
	case 3: // 3XNN, SKIP NEXT IF VX==NN
		expect := byte(opcode & 0xFF)
		if e.valueRegisters[b2] == expect {
			e.programCount += 2
		}
	case 4: // 4XNN, SKIP NEXT IF VX!=NN
		expect := byte(opcode & 0xFF)
		if e.valueRegisters[b2] != expect {
			e.programCount += 2
		}
	case 5: // 5XY0, SKIP NEXT IF VX=VY
		if e.valueRegisters[b2] == e.valueRegisters[b3] {
			e.programCount += 2
		}
	case 6: // 6XNN, SET VX=NN
		value := byte(opcode & 0xFF)
		e.valueRegisters[b2] = value
	case 7: // 7XNN, VX+=NN
		increment := byte(opcode & 0xFF)
		e.valueRegisters[b2] += increment
	case 8:
		switch b4 {
		case 0: // 8XY0, SET VX=VY
			e.valueRegisters[b2] = e.valueRegisters[b3]
		case 1: // 8XY1, VX|=VY
			e.valueRegisters[b2] |= e.valueRegisters[b3]
		case 2: // 8XY2, VX&=VY
			e.valueRegisters[b2] &= e.valueRegisters[b3]
		case 3: // 8XY3, VX^=VY
			e.valueRegisters[b2] ^= e.valueRegisters[b3]
		case 4: // 8XY4, VX+=VY
			sum, overflow := overflowAdd(e.valueRegisters[b2], e.valueRegisters[b3])
			if overflow {
				e.valueRegisters[0xF] = 1
			} else {
				e.valueRegisters[0xF] = 0
			}
			e.valueRegisters[b2] = sum
		case 5: // 8XY5, VX-=VY
			difference, borrow := overflowSubtract(e.valueRegisters[b2], e.valueRegisters[b3])
			if borrow {
				e.valueRegisters[0xF] = 0
			} else {
				e.valueRegisters[0xF] = 1
			}
			e.valueRegisters[b2] = difference
		case 6: // 8XY6, VX>>=1
			dropped := e.valueRegisters[b2] & 1
			e.valueRegisters[b2] >>= 1
			e.valueRegisters[0xF] = dropped
		case 7: // 8XY7, VX=VY-VX
			difference, borrow := overflowSubtract(e.valueRegisters[b3], e.valueRegisters[b2])
			if borrow {
				e.valueRegisters[0xF] = 0
			} else {
				e.valueRegisters[0xF] = 1
			}
			e.valueRegisters[b2] = difference
		case 0xE: // 8XYE, VX<<=1
			dropped := (e.valueRegisters[b2] >> 7) & 1
			e.valueRegisters[b2] <<= 1
			e.valueRegisters[0xF] = dropped
		default:
			return unsupportedOpcode(opcode)
		}
	case 9: // 9XY0, SKIP IF VX!=VY
		if b4 != 0 {
			return unsupportedOpcode(opcode)
		}
		if e.valueRegisters[b2] != e.valueRegisters[b3] {
			e.programCount += 2
		}
	case 0xA: // ANNN, I=NNN
		address := opcode & 0xFFF
		e.indexRegister = address
	case 0xB: // BNNN, JUMP TO V0+NNN
		offset := opcode & 0xFFF
		e.programCount = uint16(e.valueRegisters[0]) + offset
	case 0xC: // CXNN, VX=rand()&NN
		value := byte(opcode & 0xFF)
		random := byte(rand.Int())
		e.valueRegisters[b2] = random & value
	case 0xD: // DXYN, DRAW SPRITE at (VX, VY), N lines tall, sprite data at I.
		xcoord, ycoord := e.valueRegisters[b2], e.valueRegisters[b3]
		rows := b4
		flipped := false
		for yline := range rows {
			address := e.indexRegister + yline
			pixels := e.ram[address]
			for xline := range 8 {
				if pixels&(0b1000_0000>>xline) != 0 {
					x := (int(xcoord) + xline) % SCREEN_WIDTH
					y := (int(ycoord) + int(yline)) % SCREEN_HEIGHT
					index := x + SCREEN_WIDTH*y
					flipped = flipped || e.screen[index]
					e.screen[index] = !e.screen[index]
				}
			}
		}

		if flipped {
			e.valueRegisters[0xF] = 1
		} else {
			e.valueRegisters[0xF] = 0
		}
	case 0xE:
		// EX9E, SKIP IF KEY VX PRESSED
		if b3 == 9 && b4 == 0xE {
			if e.keys[e.valueRegisters[b2]] {
				e.programCount += 2
			}
		} else if b3 == 0xA && b4 == 1 { // EXA1, SKIP IF KEY VX NOT PRESSED
			if !e.keys[e.valueRegisters[b2]] {
				e.programCount += 2
			}
		} else {
			return unsupportedOpcode(opcode)
		}
	case 0xF:
		if b3 == 0 {
			if b4 == 7 { // FX07, VX=DT
				e.valueRegisters[b2] = e.delayTimer
			} else if b4 == 0xA { // FX0A, WAIT FOR KEY VX PRESS
				pressed := false
				for i, v := range e.keys {
					if v {
						e.valueRegisters[b2] = byte(i)
						pressed = true
						break
					}
				}
				if !pressed {
					e.programCount -= 2
				}
			} else {
				return unsupportedOpcode(opcode)
			}
		} else if b3 == 1 {
			switch b4 {
			case 5: // FX15, DT=VX
				e.delayTimer = e.valueRegisters[b2]
			case 8: // FX18, ST=VX
				e.soundTimer = e.valueRegisters[b2]
			case 0xE: // FX1E, I+=VX
				e.indexRegister += uint16(e.valueRegisters[b2])
			default:
				return unsupportedOpcode(opcode)
			}
		} else if b3 == 2 && b4 == 9 { // FX29, Set I to Font Address
			e.indexRegister = uint16(e.valueRegisters[b2]) * 5
		} else if b3 == 3 && b4 == 3 { // FX33, I=BCD of VX
			b := bcd.FromUint8(e.valueRegisters[b2])
			e.ram[e.indexRegister] = b & 0b100
			e.ram[e.indexRegister+1] = b & 0b010
			e.ram[e.indexRegister+2] = b & 0b001
		} else if b3 == 5 && b4 == 5 { // FX55, STORE V0-VX into I
			for index := range b2 {
				e.ram[e.indexRegister+index] = e.valueRegisters[index]
			}
		} else if b3 == 6 && b4 == 5 { // FX65, LOAD I into V0-VX
			for index := range b2 {
				e.valueRegisters[index] = e.ram[e.indexRegister+index]
			}
		} else {
			return unsupportedOpcode(opcode)
		}
	default:
		return unsupportedOpcode(opcode)
	}
	return nil
}

func unsupportedOpcode(opcode uint16) error {
	return fmt.Errorf("unsupported opcode %x", opcode)
}

func overflowAdd(a, b byte) (byte, bool) {
	if math.MaxUint8-a < b {
		return a + b, true
	}
	return a + b, false
}

func overflowSubtract(a, b byte) (byte, bool) {
	if a < b {
		return a - b, true
	}
	return a - b, false
}
