package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/fogleman/gg"
)

const mbglUrl = "https://eahazardswatch.icpac.net/mbgl-renderer/render"

var fontfile = "./fonts/OpenSans-Bold.ttf"

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

func getImageFromStyle(payload Payload, c int, r int, wg *sync.WaitGroup) {

	defer wg.Done()

	// marshal to use in post data
	mbglPayload, err := json.Marshal(payload)

	if err != nil {
		log.Fatal(err)
	}

	// send post request
	resp, err := http.Post(mbglUrl, "application/json", bytes.NewBuffer(mbglPayload))

	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	// decode image from response body
	m, _, err := image.Decode(resp.Body)

	if err != nil {
		log.Fatal(err)
	}

	result := MbglResponse{c: c, r: r, img: m}

	x := result.c*(image_width+(padding*2)) + padding + left_labels_width
	y := result.r*(image_height+(padding*2)) + padding + text_height
	dc.DrawImage(result.img, x, y)

}

var image_width int = 100
var image_height int = 100

var text_height int = 40

var left_labels_width = 70
var right_padding = 20

var padding int = 1

var dc *gg.Context

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

func generateMaps(w http.ResponseWriter, r *http.Request) {

	reqBody, err := ioutil.ReadAll(r.Body)

	if err != nil {
		log.Fatal(err)
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

	width := len(yearDekads)*(image_width+(padding*2)) + left_labels_width + right_padding
	height := (len(years) * (image_height + (padding * 2))) + (text_height * 2)

	// new empty image
	dc = gg.NewContext(width, height)
	dc.DrawRectangle(0, 0, float64(width), float64(height))

	// fill rectangle with white bg
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.Fill()

	// calculate the width for the inner rectangle. Take image width and subtract added sections and add paddings
	rec_width := (width - left_labels_width - right_padding) + padding*2

	dc.DrawRectangle(float64(left_labels_width-padding), float64(text_height-padding), float64(rec_width), float64((height-(text_height*2))+padding*2))
	dc.SetColor(color.RGBA{0, 0, 0, 100})
	dc.Fill()

	// Load font
	errr := dc.LoadFontFace(fontfile, 14)

	if errr != nil {
		log.Fatal(errr)
	}

	// set font color
	dc.SetColor(color.Black)

	// generate top labels
	for i, d := range yearDekads {
		x := float64((i * (image_width + (padding * 2))) + padding + image_width)
		y := float64(padding)

		text := fmt.Sprintf("%s %s", d.month.name, d.dekad)

		mTextWidth, mTextHeight := dc.MeasureString(text)

		x = x + (float64(image_width)-mTextWidth)/2
		y = y + (float64(text_height)-mTextHeight)/2

		dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
	}

	if err != nil {
		log.Fatal(err)
	}

	// generate left labels
	for j, year := range years {
		x := float64(padding)
		y := float64(j*(image_height+(padding*2)) + padding + text_height)

		yTextWidth, yTextHeight := dc.MeasureString(year)

		x = x + (float64(left_labels_width)-yTextWidth)/2
		y = y + (float64(image_height)-yTextHeight)/2

		dc.DrawString(year, x, y)
	}

	var wg sync.WaitGroup

	for r, row := range matrix {
		for c, val := range row {
			wg.Add(1)
			go getImageFromStyle(val, c, r, &wg)
		}
	}

	wg.Wait()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "image/png")
	dc.EncodePNG(w)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "POST":

		generateMaps(w, r)

	default:
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(http.StatusText(http.StatusNotImplemented)))
	}

}

func main() {

	// start := time.Now()

	// elapsed := time.Since(start)

	// log.Printf("Execution took %s", elapsed)

	http.HandleFunc("/render", handleGenerate)

	http.ListenAndServe(":8080", nil)

}
