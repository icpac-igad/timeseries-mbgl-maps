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

type Payload interface{}

type MbglResponse struct {
	c   int
	r   int
	img image.Image
}

type Month struct {
	name  string
	value string
}

type MonthDekad struct {
	month Month
	dekad string
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

var months []Month = []Month{
	{name: "Jan", value: "01"},
	{name: "Feb", value: "02"},
	{name: "Mar", value: "03"},
	{name: "Apr", value: "04"},
	{name: "May", value: "05"},
	{name: "Jun", value: "06"},
	{name: "Jul", value: "07"},
	{name: "Aug", value: "08"},
	{name: "Sep", value: "09"},
	{name: "Oct", value: "10"},
	{name: "Nov", value: "11"},
	{name: "Dec", value: "12"},
}

func generateMapsGrid(r *http.Request) (*gg.Context, error) {

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

	years := []string{"2019", "2020", "2021"}
	// years := []string{"1998", "1999", "2000", "2001", "2002", "2003", "2004", "2005", "2006", "2007", "2008", "2009", "2010", "2011", "2012", "2013", "2014", "2015", "2016", "2017", "2018", "2019", "2020", "2021"}

	dekads := []string{"01"}

	yearDekads := []MonthDekad{}

	for _, m := range months {
		for _, d := range dekads {
			yearDekads = append(yearDekads, MonthDekad{month: m, dekad: d})
		}

	}

	matrix := make([][]interface{}, len(years))

	for i, year := range years {
		year_data := make([]interface{}, len(months)*len(dekads))

		for k, dekad := range yearDekads {

			var tStyle Payload
			json.Unmarshal([]byte(reqBody), &tStyle)

			layerTiles := tStyle.(map[string]interface{})["style"].(map[string]interface{})["sources"].(map[string]interface{})["parameter_layer"].(map[string]interface{})["tiles"]

			tiles, _ := layerTiles.([]interface{})

			tileStr := fmt.Sprintf("%v", tiles[0])

			// replace template with real data
			tileStr = strings.Replace(tileStr, "{SELECTED_YEAR}", year, -1)
			tileStr = strings.Replace(tileStr, "{SELECTED_MONTH}", dekad.month.value, -1)
			tileStr = strings.Replace(tileStr, "{SELECTED_TENDAYS}", dekad.dekad, -1)

			tiles[0] = tileStr

			tStyle.(map[string]interface{})["style"].(map[string]interface{})["sources"].(map[string]interface{})["parameter_layer"].(map[string]interface{})["tiles"] = tiles

			year_data[k] = tStyle
		}

		matrix[i] = year_data
	}

	width := len(yearDekads)*(gridConfig.ImageWidth+(gridConfig.ImagePadding*2)) + gridConfig.LeftLabelsWidth + gridConfig.RightPadding
	height := (len(years) * (gridConfig.ImageHeight + (gridConfig.ImagePadding * 2))) + (gridConfig.TextHeight * 2)

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
	for i, d := range yearDekads {
		x := float64((i * (gridConfig.ImageWidth + (gridConfig.ImagePadding * 2))) + gridConfig.ImagePadding + gridConfig.ImageWidth)
		y := float64(gridConfig.ImagePadding)

		text := fmt.Sprintf("%s %s", d.month.name, d.dekad)

		mTextWidth, mTextHeight := dc.MeasureString(text)

		x = x + (float64(gridConfig.ImageWidth)-mTextWidth)/2
		y = y + (float64(gridConfig.TextHeight)-mTextHeight)/2

		dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
	}

	// generate left labels
	for j, year := range years {
		x := float64(gridConfig.ImagePadding)
		y := float64(j*(gridConfig.ImageHeight+(gridConfig.ImagePadding*2)) + gridConfig.ImagePadding + gridConfig.ImageHeight)

		yTextWidth, yTextHeight := dc.MeasureString(year)

		x = x + (float64(gridConfig.LeftLabelsWidth)-yTextWidth)/2
		y = y + (float64(gridConfig.ImageHeight)-yTextHeight)/2

		dc.DrawString(year, x, y)
	}

	err = getAllStyleImages(matrix, &gridConfig, dc)

	if err != nil {
		return dc, err
	}

	return dc, nil
}

func getAllStyleImages(stylesConfigMatrix [][]interface{}, gridConfig *GridConfig, dc *gg.Context) error {
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
