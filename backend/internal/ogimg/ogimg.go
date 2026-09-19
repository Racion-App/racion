// Package ogimg рисует карточку превью 1200×630 для ссылки на рецепт: фото слева, справа — домен,
// название, факты (время, ккал, цена) и кнопка-призыв. Мессенджеры и соцсети ждут 1.91:1, фото
// блюда 768×512 они режут и не показывают заголовок. Чистый Go: image/draw и шрифты Inter из embed.
package ogimg

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/Inter-Bold.ttf
var interBold []byte

//go:embed fonts/Inter-Medium.ttf
var interMedium []byte

const W, H = 1200, 630

var (
	once      sync.Once
	fBold     *opentype.Font
	fMedium   *opentype.Font
	blue      = color.RGBA{0, 122, 255, 255}
	ink       = color.RGBA{11, 11, 13, 255}
	secondary = color.RGBA{108, 108, 112, 255}
	pillBg    = color.RGBA{232, 232, 237, 255}
	bg        = color.RGBA{255, 255, 255, 255}
)

func load() {
	fBold, _ = opentype.Parse(interBold)
	fMedium, _ = opentype.Parse(interMedium)
}

func face(f *opentype.Font, size float64) font.Face {
	fc, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	return fc
}

// Card — что рисуем.
type Card struct {
	Photo  image.Image // фото блюда; nil — карточка без фото (только текст на светлом фоне)
	Domain string      // «racion.app»
	Title  string
	Facts  string // «25 мин · 404 ккал · 169 ₽»
	CTA    string // «Открыть рецепт»
}

// Render — JPEG 1200×630.
func Render(c Card) ([]byte, error) {
	once.Do(load)
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	textX, textW := 80, W-160
	if c.Photo != nil {
		// фото слева: обрезка «cover» в 560×630
		pw := 560
		cover(img, image.Rect(0, 0, pw, H), c.Photo)
		textX, textW = pw+56, W-pw-56-64
	}

	// домен — серая пилюля
	y := 96
	if c.Domain != "" {
		fm := face(fMedium, 24)
		tw := textWidth(fm, c.Domain)
		pill(img, image.Rect(textX, y, textX+tw+40, y+44), 22, pillBg)
		drawText(img, fm, c.Domain, textX+20, y+31, ink)
		y += 44 + 36
	}

	// заголовок — до трёх строк, размер подбирается под ширину
	size := 50.0
	var lines []string
	for {
		fb := face(fBold, size)
		lines = wrap(fb, c.Title, textW)
		if len(lines) <= 3 || size <= 34 {
			if len(lines) > 3 {
				lines = lines[:3]
				lines[2] = ellipsize(fb, lines[2], textW)
			}
			lineH := int(size * 1.18)
			for _, ln := range lines {
				drawText(img, fb, ln, textX, y+int(size*0.95), ink)
				y += lineH
			}
			break
		}
		size -= 4
	}

	// факты
	if c.Facts != "" {
		y += 18
		fm := face(fMedium, 26)
		drawText(img, fm, ellipsize(fm, c.Facts, textW), textX, y+24, secondary)
		y += 26 + 40
	} else {
		y += 40
	}

	// кнопка-призыв
	if c.CTA != "" {
		fb := face(fBold, 26)
		tw := textWidth(fb, c.CTA)
		if y+64 > H-64 {
			y = H - 64 - 64
		}
		pill(img, image.Rect(textX, y, textX+tw+56, y+64), 32, blue)
		drawText(img, fb, c.CTA, textX+28, y+41, color.RGBA{255, 255, 255, 255})
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 86}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// cover — вписать фото в прямоугольник с обрезкой по краям (как object-fit: cover), центрируя.
func cover(dst *image.RGBA, r image.Rectangle, src image.Image) {
	sb := src.Bounds()
	sw, sh := float64(sb.Dx()), float64(sb.Dy())
	dw, dh := float64(r.Dx()), float64(r.Dy())
	scale := dw / sw
	if sh*scale < dh {
		scale = dh / sh
	}
	cw, ch := dw/scale, dh/scale
	cx, cy := (sw-cw)/2, (sh-ch)/2
	crop := image.Rect(sb.Min.X+int(cx), sb.Min.Y+int(cy), sb.Min.X+int(cx+cw), sb.Min.Y+int(cy+ch))
	xdraw.CatmullRom.Scale(dst, r, src, crop, draw.Src, nil)
}

func drawText(dst *image.RGBA, f font.Face, s string, x, baseline int, c color.Color) {
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
}

func textWidth(f font.Face, s string) int { return font.MeasureString(f, s).Ceil() }

// wrap — перенос по словам под ширину; слишком длинное слово остаётся строкой.
func wrap(f font.Face, s string, width int) []string {
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, w := range words {
		try := w
		if cur != "" {
			try = cur + " " + w
		}
		if textWidth(f, try) <= width || cur == "" {
			cur = try
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func ellipsize(f font.Face, s string, width int) string {
	if textWidth(f, s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		if textWidth(f, strings.TrimRight(string(r), " ")+"…") <= width {
			return strings.TrimRight(string(r), " ") + "…"
		}
	}
	return "…"
}

// pill — скруглённый прямоугольник заливкой (радиус — половина высоты для пилюли).
func pill(dst *image.RGBA, r image.Rectangle, radius int, c color.Color) {
	rr := radius * radius
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dx, dy := 0, 0
			if x < r.Min.X+radius {
				dx = r.Min.X + radius - x
			} else if x >= r.Max.X-radius {
				dx = x - (r.Max.X - radius - 1)
			}
			if y < r.Min.Y+radius {
				dy = r.Min.Y + radius - y
			} else if y >= r.Max.Y-radius {
				dy = y - (r.Max.Y - radius - 1)
			}
			if dx*dx+dy*dy <= rr {
				dst.Set(x, y, c)
			}
		}
	}
}
