package console

import (
	"gopheros/device"
	"gopheros/kernel/cpu"
	"gopheros/kernel/hal/multiboot"
	"image/color"
	"reflect"
	"testing"
	"unsafe"
)

func TestVgaTextDimensions(t *testing.T) {
	cons := NewVgaTextConsole(80, 25, 0)
	if w, h := cons.Dimensions(); w != 80 || h != 25 {
		t.Fatalf("expected console dimensions to be 80x25; got %dx%d", w, h)
	}
}

func TestVgaTextDefaultColors(t *testing.T) {
	cons := NewVgaTextConsole(80, 25, 0)
	if fg, bg := cons.DefaultColors(); fg != 7 || bg != 0 {
		t.Fatalf("expected console default colors to be fg:7, bg:0; got fg:%d, bg: %d", fg, bg)
	}
}

func TestVgaTextFill(t *testing.T) {
	specs := []struct {
		// Input rect
		x, y, w, h uint16

		// Expected area to be cleared
		expX, expY, expW, expH uint16
	}{
		{
			0, 0, 500, 500,
			0, 0, 80, 25,
		},
		{
			10, 10, 11, 50,
			10, 10, 11, 15,
		},
		{
			10, 10, 110, 1,
			10, 10, 70, 1,
		},
		{
			70, 20, 20, 20,
			70, 20, 10, 5,
		},
		{
			90, 25, 20, 20,
			0, 0, 0, 0,
		},
		{
			12, 12, 5, 6,
			12, 12, 5, 6,
		},
	}

	fb := make([]uint16, 80*25)
	cons := NewVgaTextConsole(80, 25, uintptr(unsafe.Pointer(&fb[0])))
	cw, ch := cons.Dimensions()

	testPat := uint16(0xDEAD)
	clearPat := uint16(cons.clearChar)

nextSpec:
	for specIndex, spec := range specs {
		// Fill FB with test pattern
		for i := 0; i < len(fb); i++ {
			fb[i] = testPat
		}

		cons.Fill(spec.x, spec.y, spec.w, spec.h, 0, 0)

		var x, y uint16
		for y = 1; y <= ch; y++ {
			for x = 1; x <= cw; x++ {
				fbVal := fb[((y-1)*cw)+(x-1)]

				if x < spec.expX || y < spec.expY || x >= spec.expX+spec.expW || y >= spec.expY+spec.expH {
					if fbVal != testPat {
						t.Errorf("[spec %d] expected char at (%d, %d) not to be cleared", specIndex, x, y)
						continue nextSpec
					}
				} else {
					if fbVal != clearPat {
						t.Errorf("[spec %d] expected char at (%d, %d) to be cleared", specIndex, x, y)
						continue nextSpec
					}
				}
			}
		}
	}
}

func TestVgaTextScroll(t *testing.T) {
	fb := make([]uint16, 80*25)
	cons := NewVgaTextConsole(80, 25, uintptr(unsafe.Pointer(&fb[0])))
	cw, ch := cons.Dimensions()

	t.Run("up", func(t *testing.T) {
		specs := []uint16{
			0,
			1,
			2,
		}
	nextSpec:
		for specIndex, lines := range specs {
			// Fill buffer with test pattern
			var x, y, index uint16
			for y = 0; y < ch; y++ {
				for x = 0; x < cw; x++ {
					fb[index] = (y << 8) | x
					index++
				}
			}

			cons.Scroll(ScrollDirUp, lines)

			// Check that rows 1 to (height - lines) have been scrolled up
			index = 0
			for y = 0; y < ch-lines; y++ {
				for x = 0; x < cw; x++ {
					expVal := ((y + lines) << 8) | x
					if fb[index] != expVal {
						t.Errorf("[spec %d] expected value at (%d, %d) to be %d; got %d", specIndex, x, y, expVal, fb[index])
						continue nextSpec
					}
					index++
				}
			}
		}
	})

	t.Run("down", func(t *testing.T) {
		specs := []uint16{
			0,
			1,
			2,
		}

	nextSpec:
		for specIndex, lines := range specs {
			// Fill buffer with test pattern
			var x, y, index uint16
			for y = 0; y < ch; y++ {
				for x = 0; x < cw; x++ {
					fb[index] = (y << 8) | x
					index++
				}
			}

			cons.Scroll(ScrollDirDown, lines)

			// Check that rows lines to height have been scrolled down
			index = lines * cw
			for y = lines; y < ch-lines; y++ {
				for x = 0; x < cw; x++ {
					expVal := ((y - lines) << 8) | x
					if fb[index] != expVal {
						t.Errorf("[spec %d] expected value at (%d, %d) to be %d; got %d", specIndex, x, y, expVal, fb[index])
						continue nextSpec
					}
					index++
				}
			}
		}
	})
}

func TestVgaTextWrite(t *testing.T) {
	fb := make([]uint16, 80*25)
	cons := NewVgaTextConsole(80, 25, uintptr(unsafe.Pointer(&fb[0])))
	defaultFg, defaultBg := cons.DefaultColors()

	t.Run("off-screen", func(t *testing.T) {
		specs := []struct {
			x, y uint16
		}{
			{81, 26},
			{90, 24},
			{79, 30},
			{100, 100},
		}

	nextSpec:
		for specIndex, spec := range specs {
			for i := 0; i < len(fb); i++ {
				fb[i] = 0
			}

			cons.Write('!', 1, 2, spec.x, spec.y)

			for i := 0; i < len(fb); i++ {
				if got := fb[i]; got != 0 {
					t.Errorf("[spec %d] expected Write() with off-screen coords to be a no-op", specIndex)
					continue nextSpec
				}
			}
		}
	})

	t.Run("success", func(t *testing.T) {
		for i := 0; i < len(fb); i++ {
			fb[i] = 0
		}

		fg := uint8(1)
		bg := uint8(2)
		expAttr := uint16((uint16(bg) << 4) | uint16(fg))

		cons.Write('!', fg, bg, 1, 1)

		expVal := (expAttr << 8) | uint16('!')
		if got := fb[0]; got != expVal {
			t.Errorf("expected call to Write() to set fb[0] to %d; got %d", expVal, got)
		}
	})

	t.Run("fg out of range", func(t *testing.T) {
		for i := 0; i < len(fb); i++ {
			fb[i] = 0
		}

		fg := uint8(128)
		bg := uint8(2)
		expAttr := uint16((uint16(bg) << 4) | uint16(defaultFg))

		cons.Write('!', fg, bg, 1, 1)

		expVal := (expAttr << 8) | uint16('!')
		if got := fb[0]; got != expVal {
			t.Errorf("expected call to Write() to set fb[0] to %d; got %d", expVal, got)
		}
	})

	t.Run("bg out of range", func(t *testing.T) {
		for i := 0; i < len(fb); i++ {
			fb[i] = 0
		}

		fg := uint8(8)
		bg := uint8(255)
		expAttr := uint16((uint16(defaultBg) << 4) | uint16(fg))

		cons.Write('!', fg, bg, 1, 1)

		expVal := (expAttr << 8) | uint16('!')
		if got := fb[0]; got != expVal {
			t.Errorf("expected call to Write() to set fb[0] to %d; got %d", expVal, got)
		}
	})
}

func TestVgaTextSetPaletteColor(t *testing.T) {
	defer func() {
		portWriteByteFn = cpu.PortWriteByte
	}()

	cons := NewVgaTextConsole(80, 25, 0)

	t.Run("success", func(t *testing.T) {
		expWrites := []struct {
			port uint16
			val  uint8
		}{
			// Values will be normalized in the 0-31 range
			{0x3c8, 1},
			{0x3c9, 63},
			{0x3c9, 31},
			{0x3c9, 0},
		}

		writeCallCount := 0
		portWriteByteFn = func(port uint16, val uint8) {
			exp := expWrites[writeCallCount]
			if port != exp.port || val != exp.val {
				t.Errorf("[port write %d] expected port: 0x%x, val: %d; got port: 0x%x, val: %d", writeCallCount, exp.port, exp.val, port, val)
			}

			writeCallCount++
		}

		rgba := color.RGBA{R: 255, G: 127, B: 0}
		cons.SetPaletteColor(1, rgba)

		if got := cons.Palette()[1]; got != rgba {
			t.Errorf("expected color at index 1 to be:\n%v\ngot:\n%v", rgba, got)
		}

		if writeCallCount != len(expWrites) {
			t.Errorf("expected cpu.portWriteByty to be called %d times; got %d", len(expWrites), writeCallCount)
		}
	})

	t.Run("color index out of range", func(t *testing.T) {
		portWriteByteFn = func(_ uint16, _ uint8) {
			t.Error("unexpected call to cpu.PortWriteByte")
		}

		rgba := color.RGBA{R: 255, G: 127, B: 0}
		cons.SetPaletteColor(50, rgba)
	})
}

func TestVgaTextDriverInterface(t *testing.T) {
	var dev device.Driver = NewVgaTextConsole(80, 25, 0)

	if err := dev.DriverInit(); err != nil {
		t.Fatal(err)
	}

	if dev.DriverName() == "" {
		t.Fatal("DriverName() returned an empty string")
	}

	if major, minor, patch := dev.DriverVersion(); major+minor+patch == 0 {
		t.Fatal("DriverVersion() returned an invalid version number")
	}
}

func TestVgaTextProbe(t *testing.T) {
	defer func() {
		getFramebufferInfoFn = multiboot.GetFramebufferInfo
	}()

	var (
		expProbePtr = reflect.ValueOf(probeForVgaTextConsole).Pointer()
		foundProbe  bool
	)

	for _, probeFn := range HWProbes() {
		if reflect.ValueOf(probeFn).Pointer() == expProbePtr {
			foundProbe = true
			break
		}
	}

	if !foundProbe {
		t.Fatal("expected probeForVgaTextConsole to be part of the probes returned by HWProbes")
	}

	getFramebufferInfoFn = func() *multiboot.FramebufferInfo {
		return &multiboot.FramebufferInfo{
			Width:    80,
			Height:   25,
			Pitch:    160,
			PhysAddr: 0xb80000,
			Type:     multiboot.FramebufferTypeEGA,
		}
	}

	if drv := probeForVgaTextConsole(); drv == nil {
		t.Fatal("expected probeForVgaTextConsole to return a driver")
	}
}
