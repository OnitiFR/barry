package main

import (
	"github.com/OnitiFR/barry/cmd/barryd/controllers"
	"github.com/OnitiFR/barry/cmd/barryd/server"
)

// Note:
// We've made the decision to use a non-REST-like way to pass project and file
// names to the API, using query arguments instead of query paths. It makes
// thing way easier for everyone for some special cases (like the '.' project)
// and the drawbacks are very limited for us.

// AddRoutes defines all API routes for the application
func AddRoutes(app *server.App) {
	app.AddRoute(&server.Route{
		Route:   "GET /project",
		Handler: controllers.ListProjectsController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /project/files",
		Handler: controllers.ListProjectController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /project/infos",
		Handler: controllers.InfosProjectController,
	})
	app.AddRoute(&server.Route{
		Route:   "POST /project",
		Handler: controllers.ActionProjectController,
	})
	app.AddRoute(&server.Route{
		Route:   "POST /project/setting",
		Handler: controllers.SettingProjectController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /file/status",
		Handler: controllers.FileStatusController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /file/download",
		Handler: controllers.FileDownloadController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /file/push/status",
		Handler: controllers.FilePushStatusController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /key",
		Handler: controllers.ListKeysController,
	})
	app.AddRoute(&server.Route{
		Route:   "POST /key",
		Handler: controllers.NewKeyController,
	})
	app.AddRoute(&server.Route{
		Route:   "GET /status",
		Handler: controllers.GetStatusController,
	})
}
