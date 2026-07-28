package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/bitcav/nitr-core/host"
	"github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/mdp/qrterminal"
	"github.com/spf13/cobra"
)

// Every failure path below — a read error, a wrong password, mismatched new
// passwords, a database error — means the command did not do what was asked,
// so all of them return an error and exit non-zero. SilenceUsage is set at
// the top of each RunE because none of these failures is a usage mistake;
// cobra prints just the "Error:" line (the same convention the service
// commands in service.go already follow).
var Passwd = &cobra.Command{
	Use:     "passwd",
	Short:   "Changes current password.",
	Args:    cobra.NoArgs,
	PreRunE: requireNitrDB,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		var currentPassword string
		var newPassword string
		var newPasswordRepeat string
		fmt.Print("Enter current password: ")
		fmt.Println("\033[8m")
		if _, err := fmt.Scan(&currentPassword); err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println("\033[28m")
		user, err := database.GetUserByID("1")
		if err != nil {
			return err
		}

		if utils.PasswordHash(currentPassword) != user.Password {
			return errors.New("wrong password")
		}

		fmt.Print("Enter a new password: ")
		fmt.Println("\033[8m")
		if _, err := fmt.Scan(&newPassword); err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println("\033[28m")
		fmt.Print("Repeat your new password: ")
		fmt.Println("\033[8m")
		if _, err := fmt.Scan(&newPasswordRepeat); err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println("\033[28m")

		if newPassword != newPasswordRepeat {
			return errors.New("passwords don't match")
		}

		if err := database.SetUserData("1", models.User{Password: utils.PasswordHash(newPassword), Apikey: user.Apikey}); err != nil {
			return err
		}
		fmt.Println("Password changed successfully!")
		return nil
	},
}

var ApiKey = &cobra.Command{
	Use:     "key",
	Short:   "Returns the host API key",
	Args:    cobra.NoArgs,
	PreRunE: requireNitrDB,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		var password string
		fmt.Print("Enter password: ")
		fmt.Println("\033[8m")
		if _, err := fmt.Scan(&password); err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println("\033[28m")
		user, err := database.GetUserByID("1")
		if err != nil {
			return err
		}

		if utils.PasswordHash(password) != user.Password {
			return errors.New("wrong password")
		}
		fmt.Println("Your api key is:", user.Apikey)
		return nil
	},
}

var QrCode = &cobra.Command{
	Use:     "qr",
	Short:   "Prints host QR Code.",
	Args:    cobra.NoArgs,
	PreRunE: requireNitrDB,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		var password string
		fmt.Print("Enter password: ")
		fmt.Println("\033[8m")
		if _, err := fmt.Scan(&password); err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println("\033[28m")
		user, err := database.GetUserByID("1")
		if err != nil {
			return err
		}

		if utils.PasswordHash(password) != user.Password {
			return errors.New("wrong password")
		}

		apiKey, err := database.GetApiKey()
		if err != nil {
			return err
		}
		localIP, err := utils.GetLocalIP()
		if err != nil {
			return err
		}
		hostInfo := models.HostInfo{
			Name:        host.Info().Name,
			Description: host.Info().Platform + "/" + host.Info().Arch,
			IP:          localIP,
			Port:        utils.GetLocalPort(),
			Key:         apiKey,
		}

		hostInfoJSON, err := json.Marshal(hostInfo)
		if err != nil {
			return err
		}

		config := qrterminal.Config{
			Level:     qrterminal.M,
			Writer:    os.Stdout,
			BlackChar: qrterminal.WHITE,
			WhiteChar: qrterminal.BLACK,
			QuietZone: 2,
		}
		qrterminal.GenerateWithConfig(string(hostInfoJSON), config)
		return nil
	},
}
