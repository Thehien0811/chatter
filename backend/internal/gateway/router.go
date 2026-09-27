package gateway

import (
	"github.com/thehien0811/chatter/internal/gateway/app"
)

// App keeps compatibility with older imports while the app logic lives in
// the hexagonal service structure under internal/gateway/app.
type App = app.App

func NewApp(secret string) *App {
	return app.NewApp(secret)
}
