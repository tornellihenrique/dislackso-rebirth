// Package rpc serves the Connect API under /rpc. Handlers authenticate, map
// between protobuf and domain types, call a service and map errors; they hold
// no business logic.
package rpc

import (
	"net/http"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
)

// PathPrefix is where the API is mounted; the web client uses it as its base URL.
const PathPrefix = "/rpc"

const maxRequestBytes = 1 << 20

// Services are the implementations served by Mount.
type Services struct {
	Health dislacksov1connect.HealthServiceHandler
}

// Mount registers every RPC route on mux under PathPrefix.
func Mount(mux *http.ServeMux, s Services) {
	server := connect.NewServer()
	dislacksov1connect.RegisterHealthServiceHandler(server, s.Health)

	api := http.NewServeMux()
	connecthttp.Mount(api, server,
		// Requiring the protocol header forces a CORS preflight on
		// cross-origin requests, which the cookie-authenticated API relies on.
		connecthttp.WithRequireConnectProtocolHeader(),
		connecthttp.WithReadMaxBytes(maxRequestBytes),
	)
	mux.Handle(PathPrefix+"/", http.StripPrefix(PathPrefix, api))
}
