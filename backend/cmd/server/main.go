// Command server runs the Serendipity API.
package main

import (
	"net/http"

	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra"
)

func main() {
	fx.New(
		infra.App,
		fx.Invoke(func(*http.Server) {}),
	).Run()
}
