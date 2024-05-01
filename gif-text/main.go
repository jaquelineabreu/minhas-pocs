package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/fogleman/gg"
	"github.com/nfnt/resize"
)

func main() {
	imageFiles := []string{"images.jpeg", "images2.jpeg", "images3.jpeg"}

	text := "Visitar a página inicial ✅ Clicar no botão contratar ❌ Preencher numero do usuário ✅"

	var images []*image.Paletted

	for i, filename := range imageFiles {
		im, err := gg.LoadImage(filename)
		if err != nil {
			log.Fatalf("Error reading image file %s: %v", filename, err)
		}

		texts := FormatText(text)

		if i >= len(texts) {
			log.Fatalf("Not enough texts for all images")
		}

		imWithText, err := addTextAndEmoji(im, texts[i])
		if err != nil {
			log.Fatalf("Error adding text and emoji to image: %v", err)
		}

		resizedImg := resize.Resize(300, 300, imWithText, resize.Lanczos3)
		palettedImage := image.NewPaletted(resizedImg.Bounds(), palette.Plan9)
		draw.Draw(palettedImage, resizedImg.Bounds(), resizedImg, image.Point{}, draw.Src)

		images = append(images, palettedImage)
	}

	delay := 100
	generator := GIFGenerator{}

	gifBytes, err := generator.EncodeAll(&bytes.Buffer{}, images, delay)
	if err != nil {
		log.Fatalf("Error generating GIF: %v", err)
	}

	err = saveGIF(gifBytes, "output.gif")
	if err != nil {
		log.Fatalf("Error saving GIF: %v", err)
	}

	log.Println("GIF generated and saved to output.gif")
}

type GIFGenerator struct {
	Delay  int
	Frames []*image.Paletted
}

// func (g *GIFGenerator) SetEmojiIcons(emojiIcons map[string]string) {
// 	g.EmojiIcons = emojiIcons
// }

type FrameGenerated struct {
	PalettedImage *image.Paletted
	IndexFrame    int
}

func (g *GIFGenerator) generateFrame(imgByte []byte, index int, frameGenerated chan<- FrameGenerated) {
	img, _, err := image.Decode(bytes.NewReader(imgByte))
	if err != nil {
		println(imgByte[:10])
		frameGenerated <- FrameGenerated{nil, index}
		return
	}
	bounds := img.Bounds()
	palettedImage := g.Process(img)

	// Redimensiona a imagem para o tamanho desejado
	resizedImg := resize.Resize(300, 300, img, resize.Lanczos3)
	draw.Draw(palettedImage, bounds, resizedImg, bounds.Min, draw.Src)
	frameGenerated <- FrameGenerated{palettedImage, index}
}

func (g *GIFGenerator) GenerateGif(images [][]byte, delay int) ([]byte, error) {
	image.RegisterFormat("jpeg", "jpeg", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("jpg", "jpg", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)

	frameGenerated := make(chan FrameGenerated, len(images))

	for indexFrame, imgByte := range images {
		go g.generateFrame(imgByte, indexFrame, frameGenerated)
	}

	frames := make([]*image.Paletted, len(images))
	for range images {
		frame := <-frameGenerated
		if frame.PalettedImage != nil {
			frames[frame.IndexFrame] = frame.PalettedImage
		}
	}

	totalDelay := delay * len(frames)
	gitBytes, err := g.EncodeAll(&bytes.Buffer{}, frames, totalDelay)
	if err != nil {
		slog.Error("failed to encode gif")
		return nil, err
	}

	g.SetDelayAndFrames(delay, frames)

	close(frameGenerated)

	return gitBytes, nil
}

func (g *GIFGenerator) SetDelayAndFrames(delay int, frames []*image.Paletted) {
	g.Delay = delay
	g.Frames = frames
}

func (g *GIFGenerator) Process(img image.Image) *image.Paletted {
	palettedImg := image.NewPaletted(img.Bounds(), palette.Plan9)
	draw.Draw(palettedImg, img.Bounds(), img, image.Point{}, draw.Over)
	return palettedImg
}

func (g *GIFGenerator) EncodeAll(buf *bytes.Buffer, frames []*image.Paletted, delay int) ([]byte, error) {
	if len(frames) == 0 {
		slog.Error("no frames to encode")
		return nil, errors.New("no frames to encode")
	}

	gifStruct := &gif.GIF{
		Image: frames,
		Delay: make([]int, len(frames)),
	}

	for i := range gifStruct.Delay {
		gifStruct.Delay[i] = delay
	}

	err := gif.EncodeAll(buf, gifStruct)
	if err != nil {
		slog.Error("failed to encode gif")
		return nil, err
	}

	return buf.Bytes(), nil
}

func saveGIF(gifBytes []byte, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(gifBytes)
	if err != nil {
		return err
	}

	return nil
}

func addTextAndEmoji(im image.Image, text string) (image.Image, error) {
	extraSpace := 25.0

	dc := gg.NewContext(im.Bounds().Dx(), im.Bounds().Dy()+int(extraSpace))
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	dc.DrawImage(im, 0, 0)

	re := regexp.MustCompile(`(.*?)\s*([\p{So}]+)\s*`)

	match := re.FindStringSubmatch(text)
	if len(match) != 3 {
		log.Fatalf("Formato de texto inválido: %s", text)
	}

	textPart := strings.TrimSpace(match[1])
	emojiPart := strings.TrimSpace(match[2])

	fmt.Println(textPart)
	fmt.Println(emojiPart)

	emojiMap := map[string]string{
		"✅": "verifica.png",
		"❌": "fechar.png",
	}
	iconPath, ok := emojiMap[emojiPart]
	if !ok {
		log.Fatalf("Icon not found for emoji: %s", emojiPart)
	}

	icon, err := gg.LoadImage(iconPath)
	if err != nil {
		log.Fatalf("Error loading icon for emoji %s: %v", emojiPart, err)
	}

	desiredSize := 20.0
	scale := desiredSize / float64(icon.Bounds().Dx())
	iconWidth := float64(icon.Bounds().Dx()) * scale
	iconHeight := float64(icon.Bounds().Dy()) * scale

	textWidth, _ := dc.MeasureString(textPart)

	//centralizado o texto
	//textX := (im.Bounds().Dx() - int(textWidth)) / 2
	textX := 10 // Move o texto para a direita

	textY := float64(im.Bounds().Dy()) + 20

	margin := 5.0

	resizedIcon := resize.Resize(uint(iconWidth), uint(iconHeight), icon, resize.Lanczos3)

	iconX := float64(textX) + textWidth + margin

	iconY := im.Bounds().Dy() + int(extraSpace) - int(iconHeight) - 1

	dc.SetRGB(0, 0, 0)
	dc.DrawString(textPart, float64(textX), textY)

	dc.DrawImage(resizedIcon, int(iconX), iconY)

	newImage := dc.Image()

	return newImage, nil
}

func FormatText(text string) []string {

	emojiRegex := regexp.MustCompile(`[\p{So}]+`)
	emojis := emojiRegex.FindAllString(text, -1)

	parts := []string{text}
	for _, emoji := range emojis {
		var newParts []string
		for _, part := range parts {
			split := strings.Split(part, emoji)
			for i, p := range split {
				trimmedText := strings.TrimSpace(p)
				if trimmedText != "" { //
					if i < len(split)-1 {
						newParts = append(newParts, trimmedText+emoji)
					} else {
						newParts = append(newParts, trimmedText)
					}
				}
			}
		}
		parts = newParts
	}

	return parts
}
