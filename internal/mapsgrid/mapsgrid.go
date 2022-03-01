package mapsgrid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"strings"
	"sync"

	"github.com/fogleman/gg"
	"github.com/icpac-igad/timeseries-mbgl-maps/internal/conf"
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

type MapsGridPayload struct {
	XParam Param `json:"x_param,omitempty"`
	YParam Param `json:"y_param,omitempty"`
}

type MbglPayload struct {
	Width   int       `json:"width,omitempty"`
	Height  int       `json:"height,omitempty"`
	Padding int       `json:"padding,omitempty"`
	Center  []float64 `json:"center"`
	Zoom    int       `json:"zoom"`
	Bounds  []float32 `json:"bounds,omitempty"`
	Style   GlStyle   `json:"style"`
}

type MbglGridMatrix struct {
	YVal        string
	XVal        string
	MbglPayload MbglPayload
	YParamKey   string
	XParamKey   string
}

type ParamRequest struct {
	Request http.Request
	Column  int
	Row     int
}

type MbglResponse struct {
	Column int
	Row    int
	Image  image.Image
	Err    error
}

func generateRequests(done <-chan struct{}, gridMatrix [][]MbglGridMatrix) (<-chan ParamRequest, <-chan error) {

	requests := make(chan ParamRequest)
	errc := make(chan error, 1)

	go func() {
		// Close the requests channel after generator returns.
		defer close(requests)

		for irow := range gridMatrix {
			for icol, col := range gridMatrix[irow] {

				tile := col.MbglPayload.Style.Sources["parameter_layer"].Tiles[0]

				tileStr := fmt.Sprintf("%v", tile)

				// replace template with real data
				tileStr = strings.Replace(tileStr, fmt.Sprintf("{%s}", col.YParamKey), col.YVal, -1)
				tileStr = strings.Replace(tileStr, fmt.Sprintf("{%s}", col.XParamKey), col.XVal, -1)

				col.MbglPayload.Style.Sources["parameter_layer"].Tiles[0] = tileStr

				b, err := json.Marshal(col.MbglPayload)

				if err != nil {
					errc <- err
				}

				req, err := http.NewRequest("POST", conf.Configuration.MapsGrid.MbglUrl, bytes.NewBuffer(b))

				if err != nil {
					errc <- err
				}

				req.Header.Set("Content-Type", "application/json")

				select {
				case requests <- ParamRequest{Request: *req, Column: icol, Row: irow}:
				case <-done:
					errc <- fmt.Errorf("generator cancelled")
				}
			}
		}

		// no error. close error channel
		errc <- nil

	}()

	return requests, errc
}

func getMbglMapImage(done <-chan struct{}, reqs <-chan ParamRequest, c chan<- MbglResponse) {
	for req := range reqs {
		resImage, err := requestHttpImage(req.Request)

		select {
		case c <- MbglResponse{Column: req.Column, Row: req.Row, Image: resImage, Err: err}:
		case <-done:
			return
		}
	}

}

func getAllMbglMaps(gridMatrix [][]MbglGridMatrix) ([]MbglResponse, error) {
	done := make(chan struct{})
	defer close(done)

	numImages := len(gridMatrix) * len(gridMatrix[0])

	reqs, errc := generateRequests(done, gridMatrix)

	// Start a fixed number of goroutines to request urls
	c := make(chan MbglResponse) // HLc
	var wg sync.WaitGroup

	numDigesters := numImages

	wg.Add(numDigesters)

	for i := 0; i < numDigesters; i++ {
		go func() {
			getMbglMapImage(done, reqs, c) // HLc
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(c) // HLc
	}()
	// End of pipeline. OMIT

	var grids []MbglResponse

	for r := range c {
		if r.Err != nil {
			return nil, r.Err
		}

		grids = append(grids, r)
	}

	// Check whether the generator failed.
	if err := <-errc; err != nil { // HLerrc
		return nil, err
	}

	return grids, nil

}

func GetTimeseriesMaps(payload []byte) (image.Image, error) {

	// unmarshal payload
	var mapsGridPayload MapsGridPayload
	json.Unmarshal([]byte(payload), &mapsGridPayload)

	// validate params
	if mapsGridPayload.XParam.Key == "" || mapsGridPayload.YParam.Key == "" {
		return nil, fmt.Errorf("x param or y param must have a key")
	}

	if len(mapsGridPayload.XParam.Options) == 0 || len(mapsGridPayload.YParam.Options) == 0 {
		return nil, fmt.Errorf("options must have a list values")
	}

	//  generate grid matrix
	var xValues = mapsGridPayload.XParam.Options
	var yValues = mapsGridPayload.YParam.Options

	var xValuesLen = len(xValues)
	var yValuesLen = len(yValues)

	gridMatrix := make([][]MbglGridMatrix, yValuesLen)

	for i, yValue := range yValues {
		var yData = make([]MbglGridMatrix, xValuesLen)
		for k, xValue := range xValues {

			var mbglPayload MbglPayload
			json.Unmarshal([]byte(payload), &mbglPayload)

			yData[k] = MbglGridMatrix{
				YParamKey:   mapsGridPayload.YParam.Key,
				XParamKey:   mapsGridPayload.XParam.Key,
				YVal:        yValue.Value,
				XVal:        xValue.Value,
				MbglPayload: mbglPayload,
			}
		}
		gridMatrix[i] = yData
	}

	grids, err := getAllMbglMaps(gridMatrix)

	if err != nil {
		return nil, err
	}

	gridConfig := conf.Configuration.MapsGrid

	width := xValuesLen*(gridConfig.ImageWidth+(gridConfig.ImagePadding*2)) + gridConfig.LeftLabelsWidth + gridConfig.RightPadding
	height := (yValuesLen * (gridConfig.ImageHeight + (gridConfig.ImagePadding * 2))) + (gridConfig.TextHeight * 2)

	dc := gg.NewContext(width, height)
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
		return nil, err
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

	for _, grid := range grids {
		x := grid.Column*(gridConfig.ImageWidth+(gridConfig.ImagePadding*2)) + gridConfig.ImagePadding + gridConfig.LeftLabelsWidth
		y := grid.Row*(gridConfig.ImageHeight+(gridConfig.ImagePadding*2)) + gridConfig.ImagePadding + gridConfig.TextHeight
		dc.DrawImage(grid.Image, x, y)
	}

	return dc.Image(), nil
}

func requestHttpImage(req http.Request) (image.Image, error) {

	client := &http.Client{}
	res, err := client.Do(&req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	resImage, _, err := image.Decode(res.Body)

	if err != nil {
		return nil, err
	}

	return resImage, nil
}
