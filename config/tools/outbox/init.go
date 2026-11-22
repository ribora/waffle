package outbox

import (
	"time"

	"github.com/ribora/diogen/di"
	"github.com/ribora/env"
	"github.com/ribora/waffle/app"
	"github.com/ribora/waffle/tools/outbox"
	"github.com/ribora/waffle/tools/outbox/store"

	// Not decoration: diogen follows a blank import for the registrations it brings.
	// The scaffolder does not follow it for .env, so each fragment repeats what it
	// needs.
	_ "github.com/ribora/waffle/config/service/sql"
	_ "github.com/ribora/waffle/config/tools/migrate"
)

// Nothing here names a broker. A driver module supplies the outbox.Sender, and
// the table is the service's own migration, so neither belongs in this file.
func InitOutbox() {
	di.Wire[outbox.Store](store.NewStore, di.Args(map[string]any{
		"opts": []store.Option{
			store.WithMaxAttempts(env.Value("OUTBOX_MAX_ATTEMPTS", 10)),
		},
	}), di.Fallback())
	di.Define(outbox.NewPublisher, di.Alias[outbox.Publisher](), di.Fallback())

	di.Wire[outbox.Relay](outbox.NewRelay, di.Args(map[string]any{
		"opts": []outbox.RelayOption{
			outbox.WithName(env.Value("OUTBOX_RELAY_NAME", "")),
			outbox.WithPollInterval(env.Value("OUTBOX_POLL_INTERVAL", 5*time.Second)),
			outbox.WithStaleAfter(env.Value("OUTBOX_STALE_AFTER", time.Minute)),
			outbox.WithRetention(env.Value("OUTBOX_RETENTION", 7*24*time.Hour)),
			outbox.WithPurgeInterval(env.Value("OUTBOX_PURGE_INTERVAL", time.Hour)),
		},
	}), di.Fallback())

	di.Wire[app.BaseCommand](func() app.BaseCommand {
		return app.NewCommand("outbox", "Relay outbox entries to the broker")
	}, di.For[outbox.OutboxCommand]())
	di.Wire[outbox.OutboxCommand](outbox.NewOutboxCommand, di.Tag(app.CommandTag))
}
