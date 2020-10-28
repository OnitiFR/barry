package server

import (
	"fmt"
	"net/http"
)

// Request describes a request and allows to build a response
type Request struct {
	Route    *Route
	SubPath  string
	HTTP     *http.Request
	Response http.ResponseWriter
	App      *App
	APIKey   *APIKey
	// Stream          *Log
}

// Printf like helper for req.Response.Write
func (req *Request) Printf(format string, args ...interface{}) {
	req.Response.Write([]byte(fmt.Sprintf(format, args...)))
}

// Println like helper for req.Response.Write
func (req *Request) Println(message string) {
	req.Response.Write([]byte(fmt.Sprintf("%s\n", message)))
}
