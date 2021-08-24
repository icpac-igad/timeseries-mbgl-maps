package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/fogleman/gg"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

type GlStyleSource struct {
	Type_       string                 `json:"type"`
	Tiles       []string               `json:"tiles,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	TileSize    int                    `json:"tileSize,omitempty"`
	Maxzoom     int                    `json:"maxzoom,omitempty"`
	Minzoom     int                    `json:"minzoom,omitempty"`
	Url         string                 `json:"url,omitempty"`
	Attribution string                 `json:"attribution,omitempty"`
}

type GlStyle struct {
	Version int                      `json:"version"`
	Sprite  string                   `json:"sprite,omitempty"`
	Sources map[string]GlStyleSource `json:"sources"`
	Layers  []interface{}            `json:"layers"`
}

type Param struct {
	Key     string `json:"key"`
	Options []struct {
		Label string `json:"label,omitempty"`
		Value string `json:"value,omitempty"`
	} `json:"options,omitempty"`
}

type ParamsConfig struct {
	XParam Param `json:"x_param,omitempty"`
	YParam Param `json:"y_param,omitempty"`
}

type WidthHeightConfig struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

type Payload struct {
	Width   int       `json:"width,omitempty"`
	Height  int       `json:"height,omitempty"`
	Padding int       `json:"padding,omitempty"`
	Center  []float64 `json:"center"`
	Zoom    int       `json:"zoom"`
	Bounds  []float32 `json:"bounds,omitempty"`
	Style   GlStyle   `json:"style"`
}

type MbglResponse struct {
	c   int
	r   int
	img image.Image
}

type GridConfig struct {
	ImageWidth      int    `mapstructure:"ImageWidth"`
	ImageHeight     int    `mapstructure:"ImageHeight"`
	TextHeight      int    `mapstructure:"TextHeight"`
	LeftLabelsWidth int    `mapstructure:"LeftLabelsWidth"`
	RightPadding    int    `mapstructure:"RightPadding"`
	ImagePadding    int    `mapstructure:"ImagePadding"`
	FontFilePath    string `mapstructure:"FontFilePath"`
	MbglUrl         string `mapstructure:"MbglUrl"`
}

func generateMapsGrid(w http.ResponseWriter, r *http.Request) (*gg.Context, error) {
	var dc *gg.Context

	reqBody, err := ioutil.ReadAll(r.Body)

	if err != nil {
		return dc, err
	}

	var gridConfig GridConfig

	err = viper.Unmarshal(&gridConfig)

	if err != nil {
		return dc, err
	}

	var widthHeightConfig WidthHeightConfig
	json.Unmarshal([]byte(reqBody), &widthHeightConfig)

	if widthHeightConfig.Width != 0 {
		gridConfig.ImageWidth = widthHeightConfig.Width
	}

	if widthHeightConfig.Width != 0 {
		gridConfig.ImageHeight = widthHeightConfig.Width
	}

	var paramsConfig ParamsConfig
	json.Unmarshal([]byte(reqBody), &paramsConfig)

	err = checkConfig(paramsConfig)

	if err != nil {
		return dc, err
	}

	var xValues = paramsConfig.XParam.Options
	var yValues = paramsConfig.YParam.Options

	var xValuesLen = len(xValues)
	var yValuesLen = len(yValues)

	matrix := make([][]Payload, yValuesLen)

	for i, yValue := range yValues {

		var yData = make([]Payload, xValuesLen)

		for k, xValue := range xValues {

			var tStyle Payload

			json.Unmarshal([]byte(reqBody), &tStyle)

			if source, found := tStyle.Style.Sources["parameter_layer"]; found {

				tile := source.Tiles[0]

				tileStr := fmt.Sprintf("%v", tile)

				// replace template with real data
				tileStr = strings.Replace(tileStr, fmt.Sprintf("{%s}", paramsConfig.YParam.Key), yValue.Value, -1)
				tileStr = strings.Replace(tileStr, fmt.Sprintf("{%s}", paramsConfig.XParam.Key), xValue.Value, -1)

				tiles := []string{tileStr}

				source.Tiles = tiles

				tStyle.Style.Sources["parameter_layer"] = source

				yData[k] = tStyle

			}

		}

		matrix[i] = yData
	}

	width := xValuesLen*(gridConfig.ImageWidth+(gridConfig.ImagePadding*2)) + gridConfig.LeftLabelsWidth + gridConfig.RightPadding
	height := (yValuesLen * (gridConfig.ImageHeight + (gridConfig.ImagePadding * 2))) + (gridConfig.TextHeight * 2)

	// new empty image
	dc = gg.NewContext(width, height)
	dc.DrawRectangle(0, 0, float64(width), float64(height))

	// fill rectangle with white bg
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.Fill()

	// calculate the width for the inner rectangle. Take image width and subtract added sections and add paddings
	rec_width := (width - gridConfig.LeftLabelsWidth - gridConfig.RightPadding) + gridConfig.ImagePadding*2

	dc.DrawRectangle(
		float64(gridConfig.LeftLabelsWidth-gridConfig.ImagePadding),
		float64(gridConfig.TextHeight-gridConfig.ImagePadding), float64(rec_width),
		float64((height-(gridConfig.TextHeight*2))+gridConfig.ImagePadding*2),
	)

	dc.SetColor(color.RGBA{0, 0, 0, 100})
	dc.Fill()

	// Load font
	err = dc.LoadFontFace(gridConfig.FontFilePath, 14)

	if err != nil {
		return dc, err
	}

	// set font color
	dc.SetColor(color.Black)

	// generate top labels
	for i, xVal := range xValues {
		x := float64((i * (gridConfig.ImageWidth + (gridConfig.ImagePadding * 2))) + gridConfig.ImagePadding + gridConfig.ImageWidth)
		y := float64(gridConfig.ImagePadding)

		text := xVal.Label

		mTextWidth, mTextHeight := dc.MeasureString(text)

		x = x + (float64(gridConfig.ImageWidth)-mTextWidth)/2
		y = y + (float64(gridConfig.TextHeight)-mTextHeight)/2

		dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
	}

	// generate left labels
	for j, yVal := range yValues {
		x := float64(gridConfig.ImagePadding)
		y := float64(j*(gridConfig.ImageHeight+(gridConfig.ImagePadding*2)) + gridConfig.ImagePadding + gridConfig.TextHeight)

		yTextWidth, yTextHeight := dc.MeasureString(yVal.Label)

		x = x + (float64(gridConfig.LeftLabelsWidth)-yTextWidth)/2
		y = y + (float64(gridConfig.ImageHeight)-yTextHeight)/2

		dc.DrawString(yVal.Label, x, y)
	}

	err = getAllStyleImages(matrix, &gridConfig, dc)

	if err != nil {
		return dc, err
	}

	return dc, nil
}

func getAllStyleImages(stylesConfigMatrix [][]Payload, gridConfig *GridConfig, dc *gg.Context) error {
	eg, ctx := errgroup.WithContext(context.Background())
	results := make(chan MbglResponse)

	for r, row := range stylesConfigMatrix {
		for c, s := range row {
			row := r
			col := c
			tStyle := s
			eg.Go(func() error {
				img, err := getImageFromStyle(tStyle, gridConfig)
				if err != nil {
					return err
				}
				select {
				case results <- MbglResponse{c: col, r: row, img: img}:
					return nil
				// Close out if another error occurs.
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}
	}

	// Elegant way to close out the channel when the first error occurs or
	// when processing is successful.
	go func() {
		eg.Wait()
		close(results)
	}()

	for result := range results {
		x := result.c*(gridConfig.ImageWidth+(gridConfig.ImagePadding*2)) + gridConfig.ImagePadding + gridConfig.LeftLabelsWidth
		y := result.r*(gridConfig.ImageHeight+(gridConfig.ImagePadding*2)) + gridConfig.ImagePadding + gridConfig.TextHeight
		dc.DrawImage(result.img, x, y)
	}

	// Wait for all fetches to complete.
	return eg.Wait()

}

func getImageFromStyle(payload Payload, gridConfig *GridConfig) (image.Image, error) {

	var img image.Image

	// marshal to use in post data
	mbglPayload, err := json.Marshal(payload)

	if err != nil {
		return img, err
	}

	// send post request
	resp, err := http.Post(gridConfig.MbglUrl, "application/json", bytes.NewBuffer(mbglPayload))

	if err != nil {
		return img, err
	}

	defer resp.Body.Close()

	// decode image from response body
	img, _, err = image.Decode(resp.Body)

	if err != nil {
		return img, err
	}

	return img, nil
}
