// Package rpc serves the Connect API under /rpc. Handlers authenticate, map
// between protobuf and domain types, call a service and map errors; they hold
// no business logic.
package rpc

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1/dislacksov1connect"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/clientip"
)

// PathPrefix is where the API is mounted; the web client uses it as its base URL.
const PathPrefix = "/rpc"

const maxRequestBytes = 1 << 20

type Deps struct {
	Health        HealthChecker
	Auth          AuthService
	Authenticator Authenticator
	Account       AccountService
	Instance      InstanceService
	Invites       InviteService
	Setup         SetupService
	ClientIP      clientip.Resolver
	Logger        *slog.Logger
}

// Mount registers every RPC route on mux under PathPrefix.
func Mount(mux *http.ServeMux, d Deps) {
	errs := errorMapper{logger: d.Logger}
	server := connect.NewServer()
	dislacksov1connect.RegisterHealthServiceHandler(server, NewHealthHandler(d.Health))
	dislacksov1connect.RegisterAuthServiceHandler(server, &authHandler{errorMapper: errs, svc: d.Auth})
	dislacksov1connect.RegisterAccountServiceHandler(server, &accountHandler{errorMapper: errs, svc: d.Account})
	dislacksov1connect.RegisterInstanceServiceHandler(server, &instanceHandler{errorMapper: errs, svc: d.Instance, invites: d.Invites})
	dislacksov1connect.RegisterSetupServiceHandler(server, &setupHandler{errorMapper: errs, svc: d.Setup})

	api := http.NewServeMux()
	connecthttp.Mount(api, server,
		// Requiring the protocol header forces a CORS preflight on
		// cross-origin requests, which the cookie-authenticated API relies on.
		connecthttp.WithRequireConnectProtocolHeader(),
		connecthttp.WithReadMaxBytes(maxRequestBytes),
	)
	mux.Handle(PathPrefix+"/", http.StripPrefix(PathPrefix, middleware(api, d.Authenticator, d.ClientIP, d.Logger)))
}
