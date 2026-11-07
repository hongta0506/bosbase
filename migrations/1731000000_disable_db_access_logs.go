package migrations

import (
	"log/slog"

	"github.com/bosbase/bosbase-enterprise/core"
)

// Disables persisting request/access logs into the PostgreSQL `_logs` table.
//
// MaxDays = 0 makes the activityLogger middleware a no-op for every request
// (including 200 OK) and stops the batched DB log writer entirely — see
// apis.logRequest() and core's WriteFunc guard on Logs.MaxDays == 0.
// MinLevel is raised to slog.LevelError so that even if retention is
// re-enabled later, only real errors are ever written.
//
// Deploy safeguard against the 7GB / 10M-row `_logs` blowup caused by the
// crawler request loop and a flapping docker healthcheck.
//
// slog level values: Debug = -4, Info = 0, Warn = 4, Error = 8.
// (If you prefer to keep ERROR-only logs in DB for 1 day instead of none:
// set MaxDays = 1 and keep MinLevel = slog.LevelError.)
func init() {
	core.SystemMigrations.Add(&core.Migration{
		Up: func(txApp core.App) error {
			// make sure the in-memory settings mirror what is stored (or persist defaults on fresh installs)
			if err := txApp.ReloadSettings(); err != nil {
				return err
			}

			settings := txApp.Settings()
			settings.Logs.MaxDays = 0
			settings.Logs.MinLevel = int(slog.LevelError)

			// ReapplyCondition below forces this to run on every deploy, so an
			// operator can still override via the settings API afterwards.
			return txApp.Save(settings)
		},
		Down: func(txApp core.App) error {
			if err := txApp.ReloadSettings(); err != nil {
				return err
			}
			settings := txApp.Settings()
			settings.Logs.MaxDays = 5
			settings.Logs.MinLevel = int(slog.LevelInfo)
			return txApp.Save(settings)
		},
		ReapplyCondition: func(txApp core.App, runner *core.MigrationsRunner, filename string) (bool, error) {
			// reapply whenever the stored settings still allow DB access logging
			if err := txApp.ReloadSettings(); err != nil {
				return false, err
			}
			return txApp.Settings().Logs.MaxDays != 0, nil
		},
	})
}
