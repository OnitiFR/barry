package main

import (
	"github.com/OnitiFR/barry/cmd/barryd/controllers"
	"github.com/OnitiFR/barry/cmd/barryd/server"
)

// AddRoutes defines all API routes for the application
func AddRoutes(app *server.App) {
	app.AddRoute(&server.Route{
		Route:   "GET /project",
		Handler: controllers.ListProjectsController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /project/*",
		Handler: controllers.ListProjectController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /file/status/*",
		Handler: controllers.FileStatusController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /file/download/*",
		Handler: controllers.FileDownloadController,
	})
}
