package controllers

import (
	"net/http"

	"github.com/OnitiFR/barry/cmd/barryd/server"
)

// HealthCheckController answers unauthenticated liveness checks with a static
// string. It exposes nothing else: the obscurity of the (randomized) URL path
// only keeps barryd off mass scanners, it is not treated as a secret.
func HealthCheckController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "text/plain")
	req.Response.WriteHeader(http.StatusOK)
	req.Println("OK")
}
