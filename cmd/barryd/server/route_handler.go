package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/OnitiFR/barry/common"
)

// Route describes a route to a handler
type Route struct {
	Route        string
	Public       bool
	NoProtoCheck bool
	Handler      func(*Request)

	// decomposed Route
	method string
	path   string
}

func isRouteMethodAllowed(method string, methods []string) bool {
	for _, m := range methods {
		if strings.ToUpper(m) == strings.ToUpper(method) {
			return true
		}
	}
	return false
}

// extract a generic parameter from the request (API key, protocol, etc)
// from the headers (new way, "Barry-Name") or from FormValue (old way, "name")
func requestGetBarryParam(r *http.Request, name string) string {
	headerName := "Barry-" + strings.Title(name)

	val := r.Header.Get(headerName)
	if val == "" {
		val = r.FormValue(name)
	}
	return val
}

// AddRoute adds a new route to the given route muxer
func (app *App) AddRoute(route *Route) error {

	if route.Route == "" {
		return errors.New("field Route is not set")
	}

	parts := strings.Split(route.Route, " ")
	if len(parts) != 2 {
		return fmt.Errorf("invalid Route '%s'", route.Route)
	}

	method := parts[0]
	switch method {
	case "GET":
	case "PUT":
	case "POST":
	case "DELETE":
	case "OPTIONS":
	case "HEAD":
	default:
		return fmt.Errorf("unsupported method '%s'", method)
	}

	// remove * (if any) at the end of route path
	path := strings.TrimRight(parts[1], "*")
	if path == "" {
		return errors.New("field Route path is invalid")
	}

	route.method = method
	route.path = path

	app.routesAPI[path] = append(app.routesAPI[path], route)

	return nil
}

func (app *App) registerRouteHandlers(mux *http.ServeMux, inRoutes map[string][]*Route) {
	for _path, _routes := range inRoutes {
		// capture _path, _routes in the closure
		go func(path string, routes []*Route) {
			// look for duplicated methods in routes
			methods := make(map[string]bool)
			for _, route := range routes {
				_, exists := methods[route.method]
				if exists {
					log.Fatalf("router: duplicated method '%s' for path '%s'", route.method, path)
				}
				methods[route.method] = true
			}

			mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				ip, _, _ := net.SplitHostPort(r.RemoteAddr)
				app.Log.Tracef(MsgGlob, "HTTP call: %s %s %s [%s]", ip, r.Method, path, r.UserAgent())

				var validRoute *Route
				for _, route := range routes {
					if route.method == r.Method {
						validRoute = route
					}
				}

				if validRoute == nil {
					errMsg := fmt.Sprintf("Method was %s for route %s", r.Method, path)
					app.Log.Errorf(MsgGlob, "%d: %s", 405, errMsg)
					http.Error(w, errMsg, 405)
					return
				}
				routeHandleFunc(validRoute, w, r, app)
			})
		}(_path, _routes)
	}
}

func routeHandleFunc(route *Route, w http.ResponseWriter, r *http.Request, app *App) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Latest-Known-Client-Version", common.ClientVersion)

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	if route.NoProtoCheck == false {
		clientProto, _ := strconv.Atoi(requestGetBarryParam(r, "protocol"))
		if clientProto != ProtocolVersion {
			errMsg := fmt.Sprintf("Protocol mismatch, server requires version %d", ProtocolVersion)
			app.Log.Errorf(MsgGlob, "%d: %s", 400, errMsg)
			http.Error(w, errMsg, 400)
			return
		}
	}

	// extract relative path
	subPath := r.URL.Path[len(route.path):]

	request := &Request{
		Route:    route,
		SubPath:  subPath,
		HTTP:     r,
		Response: w,
		App:      app,
	}

	if route.Public == false {
		valid, key := app.APIKeysDB.IsValidKey(requestGetBarryParam(r, "key"))
		if valid == false {
			errMsg := "invalid key"
			app.Log.Errorf(MsgGlob, "%d: %s", 403, errMsg)
			http.Error(w, errMsg, 403)
			return
		}
		request.APIKey = key
		app.Log.Tracef(MsgGlob, "API call: %s %s %s (key: %s)", ip, r.Method, route.path, key.Comment)
	} else {
		app.Log.Tracef(MsgGlob, "API call: %s %s %s", ip, r.Method, route.path)
	}

	route.Handler(request)
}
