// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package serve

import (
	"github.com/spf13/cobra"

	"github.com/ory/kratos/cmd/daemon"
	"github.com/ory/kratos/driver"
	"github.com/ory/kratos/driver/config"
	"github.com/ory/x/configx"
)

// NewServeCmd returns the serve command
func NewServeCmd(dOpts ...driver.RegistryOption) (serveCmd *cobra.Command) {
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Run the Ory Kratos server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			d, err := driver.New(ctx, cmd.ErrOrStderr(), append(dOpts, driver.WithConfigOptions(configx.WithFlags(cmd.Flags())))...)
			if err != nil {
				return err
			}

			if d.Config().IsInsecureDevMode(ctx) {
				d.Logger().Warn(`

YOU ARE RUNNING Ory KRATOS IN DEV MODE.
SECURITY IS DISABLED.
DON'T DO THIS IN PRODUCTION!

`)
			}

			configVersion := d.Config().ConfigVersion(ctx)
			if configVersion == config.UnknownVersion {
				d.Logger().Warn("The config has no version specified. Add the version to improve your development experience.")
			} else if config.Version != "" &&
				configVersion != config.Version {
				d.Logger().Warnf("Config version is '%s' but kratos runs on version '%s'", configVersion, config.Version)
			}

			return daemon.ServeAll(d)(cmd, args)
		},
	}
	configx.RegisterFlags(serveCmd.PersistentFlags())

	serveCmd.PersistentFlags().Bool("sqa-opt-out", false, "Disable anonymized telemetry reports - for more information please visit https://www.ory.sh/docs/ecosystem/sqa")
	serveCmd.PersistentFlags().Bool("dev", false, "Disables critical security features to make development easier")
	serveCmd.PersistentFlags().Bool("watch-courier", false, "Run the message courier as a background task, to simplify single-instance setup")
	return serveCmd
}

func RegisterCommandRecursive(parent *cobra.Command, dOpts []driver.RegistryOption) {
	parent.AddCommand(NewServeCmd(dOpts...))
}
