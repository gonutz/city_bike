package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"strings"

	"github.com/gonutz/prototype/draw"
	"github.com/gonutz/ease"
)

//go:embed rsc
var fileSystem embed.FS

var frontYardColor = rgb(38, 38, 38)

type game struct {
	window     draw.Window
	windowW    int
	windowH    int
	state      gameState
	scale      float64
	camDx      float64
	camDy      float64
	fade       float32
	camSpeedY  float64
	zoomTimer int
}

type gameState int

const (
	loadingAssets gameState = iota
	fadingInMenu
	fadingOutMenu
	fadingInGame
	ascendingIntoGame
	zoomingIntoGame
)

func (g *game) update(window draw.Window) {
	if window.WasKeyPressed(draw.KeyEscape) ||
		strings.Contains(window.Characters(), "ö") {
		// TODO Remove debug code:
		window.Close()
	}

	window.BlurImages(false)

	g.window = window
	g.windowW, g.windowH = window.Size()

	if g.state == loadingAssets {
		g.init()
	} else if g.state == fadingInMenu || g.state == fadingOutMenu {
		g.menu()
	} else {
		g.run()
	}
}

func (g *game) init() {
	g.state = fadingInMenu

	files, err := fileSystem.ReadDir("rsc")
	check(err)
	for _, file := range files {
		name := file.Name()
		if strings.HasSuffix(name, ".png") {
			_, _, err := g.window.ImageSize(name)
			if err == draw.ErrImageLoading {
				g.state = loadingAssets
			} else if err != nil {
				panic("failed to load " + name + ": " + err.Error())
			}
		}
	}

	if g.state == fadingInMenu {
		g.window.SetFullscreen(true)
		g.window.ShowCursor(false)
		g.fade = 1.1
	}
}

func (g *game) menu() {
	mustStart := false

	mouseX, mouseY := g.window.MousePosition()
	startW, startH := g.size("start_button")
	scale := g.windowH / 100
	startW *= scale
	startH *= scale
	startX := (g.windowW - startW) / 2
	startY := (g.windowH - startH) / 2
	startTint := draw.Tint(draw.RGB(0.5, 0.5, 0.5))
	if startX <= mouseX && mouseX < startX+startW &&
		startY <= mouseY && mouseY < startY+startH {
		startTint = draw.Tint(draw.White)
		if len(g.window.Clicks()) > 0 {
			mustStart = true
		}
	}
	check(g.window.DrawImage("start_button.png", draw.At(startX, startY), draw.Scale(scale), startTint))
	check(g.window.DrawImage("cursor.png", draw.At(mouseX-4, mouseY), draw.Scale(scale)))

	if g.window.WasKeyPressed(draw.KeySpace) ||
		g.window.WasKeyPressed(draw.KeyEnter) ||
		g.window.WasKeyPressed(draw.KeyNumEnter) ||
		g.window.WasKeyPressed(draw.KeyEscape) {
		mustStart = true
	}

	if mustStart && g.state != fadingOutMenu {
		g.state = fadingOutMenu
	}

	if g.state == fadingInMenu {
		g.fade = max(0, g.fade-0.01)
	} else if g.state == fadingOutMenu {
		g.fade += 0.01
		if g.fade >= 1 {
			g.state = fadingInGame
			g.fade = 1.4
			g.scale = 3
			g.camDx = -100
			g.camDy = 300
		}
	}
	a := max(0, min(1, g.fade))
	g.window.FillRect(0, 0, g.windowW, g.windowH, draw.RGBA(0, 0, 0, a))
}

func (g *game) run() {
	g.scale = max(1, g.scale+g.window.MouseWheelY()*0.23847) // TODO

	if g.window.IsKeyDown(draw.KeyLeft) || g.window.IsKeyDown(draw.KeyH) || g.window.IsKeyDown(draw.KeyA) {
		g.camDx += 2
	}
	if g.window.IsKeyDown(draw.KeyRight) || g.window.IsKeyDown(draw.KeyL) || g.window.IsKeyDown(draw.KeyD) {
		g.camDx -= 2
	}
	if g.window.IsKeyDown(draw.KeyUp) || g.window.IsKeyDown(draw.KeyK) || g.window.IsKeyDown(draw.KeyW) {
		g.camDy += 1
	}
	if g.window.IsKeyDown(draw.KeyDown) || g.window.IsKeyDown(draw.KeyJ) || g.window.IsKeyDown(draw.KeyS) {
		g.camDy -= 1
	}

	g.camDx = min(0, g.camDx)
	g.camDy = max(0, g.camDy)

	visibleLeft := max(0, round(-g.camDx-0.51))
	visibleWidth := round(float64(g.windowW)/g.scale+0.51) + 1
	visibleRight := visibleLeft + visibleWidth - 1

	visibleBottom := max(0, round(g.camDy-0.51))
	visibleHeight := round(float64(g.windowH)/g.scale+0.51) + 1
	visibleTop := visibleBottom + visibleHeight - 1
	_ = visibleTop // TODO

	streetW, streetH := g.size("street")
	fenceW, fenceH := g.size("fence")
	skyscraperW, _ := g.size("skyscraper_0")
	frontYardH := fenceH + 1
	lampDx := streetW + 30

	// Draw the world.

	_, skyY := g.worldToScreen(0, 300)
	g.window.FillRectTint(0, skyY, g.windowW, g.windowH, [4]draw.Color{
		rgb(12, 19, 34),
		rgb(12, 19, 34),
		rgb(36, 34, 48),
		rgb(36, 34, 48),
	})
	g.window.FillRect(0, 0, g.windowW, skyY, rgb(12, 19, 34))

	for x := visibleLeft; x < visibleRight; x++ {
		if x%3 == 0 {
			starX, starY := g.worldToScreen(x, 250+randStarDy(x))
			g.window.FillRect(starX, starY, 1, 1, rgb(255, 255, 200))
		}
	}

	for x := visibleLeft - 20; x < visibleRight+20; x++ {
		if x%15 == 0 {
			s := randBackSkyscraper(x)
			g.draw(s.imageName, x+s.dx, streetH+120+s.dy, s.tint)
		}
	}

	g.fillRect(visibleLeft, streetH, visibleWidth, frontYardH, frontYardColor)

	gapDx := skyscraperW - 1
	gapI := visibleLeft / gapDx
	gapX := gapI * gapDx
	for gapX < visibleRight+gapDx {
		isGap := randIsGap(gapI)

		if isGap {
			g.fillRect(gapX, streetH, gapDx, 130, rgb(38, 56, 34))
			g.draw("grass", gapX+10, streetH+19)
			g.draw("grass", gapX+30, streetH+40)
			g.draw("grass", gapX+20, streetH+53)
			g.draw("grass", gapX+45, streetH+61)
			g.draw("grass", gapX+5, streetH+74)
			g.draw("grass", gapX+37, streetH+87)
			g.draw("grass", gapX+30, streetH+110)
			switch randGapType(gapI) {
			case 0:
				g.draw("tree_0", gapX-17, streetH+80)
				g.draw("tree_1", gapX+31, streetH+52)
				g.draw("tree_0", gapX-6, streetH+43)
			case 1:
				g.draw("tree_0", gapX+15, streetH+80)
				g.draw("tree_1", gapX-15, streetH+59)
				g.draw("tree_0", gapX+40, streetH+43)
				g.draw("tree_1", gapX+12, streetH+13)
			case 2:
				g.draw("tree_1", gapX+3, streetH+62)
				g.draw("tree_0", gapX+20, streetH+26)
			}
		}

		gapI++
		gapX += gapDx
	}

	skyscraperDx := skyscraperW - 1
	skyscraperI := visibleLeft / skyscraperDx
	skyscraperX := skyscraperI * skyscraperDx
	for skyscraperX < visibleRight+skyscraperDx {
		isGap := randIsGap(skyscraperI)

		if !isGap {
			img := randSkyscraper(skyscraperI)
			tint := randSkyscraperTint(skyscraperI)
			g.draw(img, skyscraperX, streetH, tint)

			for _, item := range randBushesAndTrashCans(skyscraperI) {
				g.draw(item.imageName, skyscraperX+item.dx, streetH+item.dy)
			}
		}

		skyscraperI++
		skyscraperX += skyscraperDx
	}

	topFenceI := visibleLeft / fenceW
	topFenceX := topFenceI * fenceW
	for topFenceX < visibleRight {
		img := randFenceDoor(topFenceI)
		g.draw(img, topFenceX, streetH)
		topFenceI++
		topFenceX += fenceW
	}

	streetX := visibleLeft / streetW * streetW
	for streetX < visibleRight {
		g.draw("street", streetX, 0)
		streetX += streetW
	}

	bottomFenceX := visibleLeft / fenceW * fenceW
	for bottomFenceX < visibleRight {
		g.draw("fence", bottomFenceX, 0)
		bottomFenceX += fenceW
	}

	lampOffsetX := -15
	topLampX := visibleLeft/lampDx*lampDx + lampOffsetX
	for topLampX < visibleRight {
		g.draw("lamp_top", topLampX, 26)
		topLampX += lampDx
	}

	bottomLampX := visibleLeft/lampDx*lampDx + lampOffsetX
	for bottomLampX < visibleRight {
		g.draw("lamp_bottom", bottomLampX+16, 7)
		bottomLampX += lampDx
	}

	if g.state == fadingInGame {
		g.fade -= 0.01
		a := max(0, min(1, g.fade))
		g.window.FillRect(0, 0, g.windowW, g.windowH, draw.RGBA(0, 0, 0, a))
		if g.fade < -0.3 {
			g.state = ascendingIntoGame
		}
	}

	if g.state == ascendingIntoGame {
		if g.camDy > 150 {
			g.camSpeedY -= 0.02
		} else {
			g.camSpeedY = min(-0.1, g.camSpeedY+0.02)
		}
		g.camDy += g.camSpeedY
		if g.camDy < 0 {
			g.camDy = 0
			g.state = zoomingIntoGame
		}
	}

	if g.state == zoomingIntoGame {
		before := float64(g.windowW) / g.scale

		g.zoomTimer++
		g.scale = 3 + ease.InOutQuad(float64(g.zoomTimer)*0.01)*10

		after := float64(g.windowW) / g.scale

		g.camDx += (after - before) / 2
	}
}

func randStarDy(i int) int {
	return rand.New(rand.NewSource(int64(i))).Intn(1200)
}

func randFenceDoor(i int) string {
	loop := []int{0, 2, 0, 1, 0, 2, 1, 2, 0, 2, 1}
	return fmt.Sprintf("fence_door_%d", loop[i%len(loop)])
}

func randIsGap(i int) bool {
	loop := "    x      x           x                 x      x         x     "
	return loop[i%len(loop)] == 'x'
}

func randGapType(i int) int {
	loop := []int{0, 1, 2, 0, 2, 1, 0, 1, 0, 2, 0, 1}
	return loop[i%len(loop)]
}

func randSkyscraper(i int) string {
	loop := []int{0, 1, 2, 1, 2, 0, 2, 1, 0, 1, 2, 0, 1}
	return fmt.Sprintf("skyscraper_%d", loop[i%len(loop)])
}

func randSkyscraperTint(i int) draw.DrawImageOption {
	loop := []int{45, 57, 54, 63, 48, 60, 51}
	perc := loop[i%len(loop)]
	v := float32(perc) / 100
	return draw.Tint(draw.RGB(v, v, v))
}

func randBackSkyscraper(i int) backSkyscraper {
	if i < 0 {
		i = -i
	}
	r := rand.New(rand.NewSource(int64(i)))
	nameLoop := []int{2, 1, 0, 2, 0, 1, 0, 2, 0, 2, 1, 0, 2, 1, 2, 0}
	a := 0.4 + 0.1*r.Float32()
	return backSkyscraper{
		imageName: fmt.Sprintf("background_skyscraper_%d", nameLoop[i%len(nameLoop)]),
		dx:        -5 + r.Intn(10),
		dy:        -r.Intn(25),
		tint:      draw.Tint(draw.RGB(a, a, a)),
	}
}

type backSkyscraper struct {
	imageName string
	dx        int
	dy        int
	tint      draw.DrawImageOption
}

func randBushesAndTrashCans(i int) []drawItem {
	loop := []int{4, 1, 0, 2, 4, 3, 1, 0, 2, 3, 2, 1, 2, 3, 1, 4, 2, 3}
	kind := loop[i%len(loop)]
	switch kind {
	case 0:
		return []drawItem{
			{imageName: "trashcan", dx: -4, dy: 5},
			{imageName: "trashcan", dx: -10, dy: 4},
			{imageName: "trashcan", dx: 5, dy: 3},
		}
	case 1:
		return []drawItem{
			{imageName: "bush_0", dx: -10, dy: 4},
			{imageName: "bush_1", dx: 5, dy: 3},
		}
	case 2:
		return []drawItem{
			{imageName: "bush_1", dx: 10, dy: 4},
			{imageName: "trashcan", dx: -8, dy: 4},
			{imageName: "trashcan", dx: 5, dy: 2},
		}
	case 3:
		return []drawItem{
			{imageName: "bush_1", dx: 10, dy: 4},
			{imageName: "bush_1", dx: -9, dy: 5},
			{imageName: "bush_0", dx: 2, dy: 3},
		}
	case 4:
		return []drawItem{
			{imageName: "trashcan", dx: 8, dy: 4},
			{imageName: "bush_0", dx: -5, dy: 3},
		}
	}
	return nil
}

type drawItem struct {
	imageName string
	dx        int
	dy        int
}

func (g *game) size(imageName string) (int, int) {
	img := imageName + ".png"
	w, h, err := g.window.ImageSize(img)
	check(err)
	return w, h
}

func (g *game) fillRect(x, y, w, h any, c draw.Color) {
	g.window.FillRect(
		round((g.camDx+toFloat64(x))*g.scale),
		round(g.camDy*g.scale+float64(g.windowH)-g.scale*(toFloat64(y)+toFloat64(h))),
		round(toFloat64(w)*g.scale),
		round(toFloat64(h)*g.scale),
		c,
	)
}

func (g *game) worldToScreen(x, y any) (int, int) {
	screenX := round((g.camDx + toFloat64(x)) * g.scale)
	screenY := round(g.camDy*g.scale + float64(g.windowH) - g.scale*(toFloat64(y)))
	return screenX, screenY
}

func (g *game) draw(imageName string, x, y any, opt ...draw.DrawImageOption) {
	img := imageName + ".png"

	_, imageH, err := g.window.ImageSize(img)
	check(err)

	at := draw.At(
		(g.camDx+toFloat64(x))*g.scale,
		g.camDy*g.scale+float64(g.windowH)-g.scale*(toFloat64(y)+float64(imageH)),
	)
	opt = append(opt, at, draw.Scale(g.scale))
	g.window.DrawImage(img, opt...)
}

func main() {
	rsc, err := fs.Sub(fileSystem, "rsc")
	check(err)

	draw.OpenFile = func(path string) (io.ReadCloser, error) {
		return rsc.Open(path)
	}

	g := game{
		scale: 5,
	}

	draw.RunWindow("City Bike", 1500, 800, func(window draw.Window) {
		g.update(window)
	})
}

func round(x float64) int {
	if x < 0 {
		return int(x - 0.5)
	}
	return int(x + 0.5)
}

func rgb(r, g, b byte) draw.Color {
	return draw.RGB(
		float32(r)/255,
		float32(g)/255,
		float32(b)/255,
	)
}

func toFloat64(x any) float64 {
	switch x := x.(type) {
	case int:
		return float64(x)
	case float32:
		return float64(x)
	case float64:
		return float64(x)
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case complex64:
		return float64(real(x))
	case complex128:
		return float64(real(x))
	}
	return 0
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
