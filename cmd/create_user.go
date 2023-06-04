package cmd

import (
	"context"

	"github.com/oxygenpay/oxygen/internal/app"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var createUserCommand = &cobra.Command{
	Use:     "create-user",
	Short:   "Creates new user with provided email & password",
	Args:    cobra.ExactArgs(2),
	Example: "oxygen create-user user@gmail.com qwerty123",
	Run:     createUser,
}

var overridePassword bool

func createUser(_ *cobra.Command, args []string) {
	var (
		ctx         = context.Background()
		cfg         = resolveConfig()
		service     = app.New(ctx, cfg)
		users       = service.Locator().UserService()
		logger      = service.Logger()
		email, pass = args[0], args[1]
	)

	u, err := users.Register(ctx, email, pass)

	switch {
	case errors.Is(err, user.ErrAlreadyExists):
		if !overridePassword {
			logger.Info().Msg("User already exists. Use --override-password option to update password")
			return
		}

		if _, err = users.UpdatePassword(ctx, u.ID, pass); err != nil {
			logger.Error().Err(err).Msg("User already exists. Unable to update password")
			return
		}

		logger.Info().Msg("User already exists. Updated password")
	case err != nil:
		logger.Err(err).Msg("Unable to create user")
	default:
		logger.Info().Msg("User created")
	}
}
