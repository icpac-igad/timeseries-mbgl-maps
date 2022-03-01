package service

import (
	"bytes"
	"fmt"
	"image/png"
	"io/ioutil"
	"net/http"

	"github.com/gocraft/web"
	"github.com/icpac-igad/timeseries-mbgl-maps/internal/mapsgrid"
)

type Context struct {
}

func Error(rw web.ResponseWriter, req *web.Request, err interface{}) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("recovered panic:", err)
			return
		}
		fmt.Println("no panic recovered")
	}()
}

func (c *Context) HandleGetTimeSeries(rw web.ResponseWriter, req *web.Request) {

	// ready request body
	body, err := ioutil.ReadAll(req.Body)

	defer req.Body.Close()

	if err != nil {
		err := appError{Status: http.StatusBadRequest, Message: err.Error()}
		JSONHandleError(rw, err)
		return
	}

	// generate timeseries collage
	gridImage, err := mapsgrid.GetTimeseriesMaps(body)

	if err != nil {
		err := appError{Status: http.StatusBadRequest, Message: err.Error()}
		JSONHandleError(rw, err)
		return
	}

	buff := new(bytes.Buffer)
	err = png.Encode(buff, gridImage)

	if err != nil {
		err := appError{Status: http.StatusBadRequest, Message: "Error encoding grid image"}
		JSONHandleError(rw, err)
		return
	}

	rw.Header().Set("Content-Type", "image/png")
	rw.Write(buff.Bytes())

}

func initRouter(basePath string) *web.Router {
	// create router
	router := web.New(Context{})

	// ovveride gocraft defualt error handler
	router.Error(Error)

	// add middlewares
	router.Middleware(web.LoggerMiddleware)
	// router.Middleware(web.ShowErrorsMiddleware)

	// handle routes
	router.Post("/", (*Context).HandleGetTimeSeries)

	return router
}
