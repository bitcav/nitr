package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// RunServer starts the nitr server (the host service). main assigns it
// before dispatching, so flag-carrying invocations like
// `nitr --port 9000` reach the server through rootCmd's RunE. It stays nil
// in this package's tests, where a bare `nitr` must print help and
// provision nothing.
var RunServer func() error

var rootCmd = &cobra.Command{
	Use:   "nitr",
	Short: "Nitr is a remote monitoring tool for system information gathering.",
	// PersistentPreRun re-establishes the viper flag bindings on every
	// invocation: viper.Reset() (called freely in tests, and harmless in
	// production) wipes them, and binding lazily at Get time is what makes
	// the flags > env > file > default precedence work.
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		bindConfigFlags(cmd.Root().PersistentFlags())
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		if RunServer == nil {
			return cmd.Help()
		}
		return RunServer()
	},
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.String("config", "", `path to the config file (default "config.ini" in the working directory; parsed as YAML despite the extension)`)
	flags.String("port", "", `port to listen on (env NITR_PORT, config key "port", default 8000)`)
	flags.String("host", "", `address to bind (env NITR_BIND_ADDRESS, config key "bind_address", default "0.0.0.0" = all interfaces; "127.0.0.1" for localhost only)`)
	flags.String("data-dir", "", `directory holding nitr.db (env NITR_DATA_DIR, config key "data_dir", default working directory)`)
	// --bind is accepted as an alias of --host by normalizing the name
	// during parsing, so both share the one flag (and one viper binding).
	rootCmd.SetGlobalNormalizationFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "bind" {
			name = "host"
		}
		return pflag.NormalizedName(name)
	})
	rootCmd.AddCommand(VersionCmd)
	rootCmd.AddCommand(ApiKey)
	rootCmd.AddCommand(Passwd)
	rootCmd.AddCommand(QrCode)
	rootCmd.AddCommand(lifecycleCmds...)
}

// bindConfigFlags wires the persistent flags into viper under their config
// key names. BindPFlag only fails when the flag name is wrong (a build-time
// fact), so the error is safe to discard.
func bindConfigFlags(flags *pflag.FlagSet) {
	for key, name := range map[string]string{
		"config":       "config",
		"port":         "port",
		"bind_address": "host",
		"data_dir":     "data-dir",
	} {
		_ = viper.BindPFlag(key, flags.Lookup(name))
	}
}

func Execute() {
	ExecuteArgs(os.Args[1:])
}

// ExecuteArgs runs the root command with an explicit argv (no program name).
// Exported so main.dispatch can forward the routed args into cobra instead of
// letting cobra fall back to the real os.Args — which is what made the old
// dispatch test a no-op (it parsed the go-test binary's -test.* flags).
//
// It deliberately performs no provisioning of its own: informational
// commands (version, -h, help, unknown) leave the working directory
// untouched, and the credential-using subcommands (key, passwd, qr) refuse
// to run unless nitr.db already exists (see requireNitrDB). main.server is
// the only path that creates nitr.db or config.ini — running "nitr version",
// "nitr -h", or an unknown command must stay side-effect free, since
// "nitr version" is what install scripts and CI smoke tests run, and
// provisioning a default "123456" user from an informational command is not
// acceptable.
func ExecuteArgs(args []string) {
	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// requireNitrDB is the PreRunE for the subcommands that read or write the
// credential store (ApiKey, Passwd, QrCode). It refuses to run unless
// nitr.db already exists at its resolved location (database.DBPath — the
// --data-dir flag / NITR_DATA_DIR env / data_dir key, else the working
// directory): silently minting an empty credential store is how a user ends
// up with a second database that diverges from the one the server actually
// uses. main.server remains the only thing that creates nitr.db.
//
// Once the database is present, the config file is loaded into viper so
// commands like `qr` pick up the configured port. config.ini is created by
// the same server setup that creates nitr.db, so loading it here never
// provisions anything that wasn't already there.
func requireNitrDB(cmd *cobra.Command, _ []string) error {
	dbPath := database.DBPath()
	if _, err := os.Stat(dbPath); err != nil {
		// SilenceUsage on this specific error only: the command was typed
		// correctly and printing usage would tell the user to look for a
		// syntax mistake that does not exist, burying the actionable message
		// under boilerplate. Set on the command, not in the struct literal,
		// so genuine usage errors (unknown flag, bad arg) still print usage.
		cmd.SilenceUsage = true
		where := dbPath
		if !filepath.IsAbs(where) {
			if cwd, err := os.Getwd(); err == nil {
				where = filepath.Join(cwd, where)
			}
		}
		return fmt.Errorf("no nitr database found at %s — start the nitr server there first", where)
	}
	utils.ConfigFileSetup()
	return nil
}
