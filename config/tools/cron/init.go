package cron

import (
	"github.com/ribora/diogen/di"
	"github.com/ribora/env"
	"github.com/ribora/waffle/app"
	"github.com/ribora/waffle/tools/cron"
)

// CRON_SCHEDULE is newline-separated, not comma-separated — comma is reserved
// for cron's own list-of-values syntax (e.g. "0 8,20 * * *").
func InitCron() {
	di.Wire[app.BaseCommand](func() app.BaseCommand { return app.NewCommand("cron", "Run scheduled commands") }, di.For[cron.CronCommand]())
	di.Wire[cron.CronCommand](cron.NewCronCommand, di.Tag(app.CommandTag), di.Args(map[string]any{
		"schedule": env.Value("CRON_SCHEDULE", ""),
		"timezone": env.Value("TZ", "UTC"),
	}))
}
