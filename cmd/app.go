package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bitcav/nitr-core/host"
	"github.com/bitcav/nitr/database"
	"github.com/bitcav/nitr/models"
	"github.com/bitcav/nitr/utils"
	"github.com/mdp/qrterminal"
	"github.com/spf13/cobra"
)

var Passwd = &cobra.Command{
	Use:   "passwd",
	Short: "Changes current password.",
	Run: func(cmd *cobra.Command, args []string) {
		var currentPassword string
		var newPassword string
		var newPasswordRepeat string
		fmt.Print("Enter current password: ")
		fmt.Println("\033[8m")
		fmt.Scan(&currentPassword)
		fmt.Println("\033[28m")
		user := database.GetUserByID("1")

		if utils.PasswordHash(currentPassword) == user.Password {
			fmt.Print("Enter a new password: ")
			fmt.Println("\033[8m")
			fmt.Scan(&newPassword)
			fmt.Println("\033[28m")
			fmt.Print("Repeat your new password: ")
			fmt.Println("\033[8m")
			fmt.Scan(&newPasswordRepeat)
			fmt.Println("\033[28m")
			if newPassword == newPasswordRepeat {
				user := models.User{Password: utils.PasswordHash(newPassword), Apikey: user.Apikey}
				err := database.SetUserData("1", user)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Println("Password changed succesfully!")
			} else {
				fmt.Println("Passwords don't match")

			}

		} else {
			fmt.Println("Wrong password.")
		}
	},
}

var ApiKey = &cobra.Command{
	Use:   "key",
	Short: "Returns the host API key",
	Run: func(cmd *cobra.Command, args []string) {
		var password string
		fmt.Print("Enter password: ")
		fmt.Println("\033[8m")
		fmt.Scan(&password)
		fmt.Println("\033[28m")
		user := database.GetUserByID("1")

		if utils.PasswordHash(password) == user.Password {
			fmt.Println("Your api key is:", user.Apikey)
		} else {
			fmt.Println("Wrong password.")
		}
	},
}

var QrCode = &cobra.Command{
	Use:   "qr",
	Short: "Prints host QR Code.",
	Run: func(cmd *cobra.Command, args []string) {
		var password string
		fmt.Print("Enter password: ")
		fmt.Println("\033[8m")
		fmt.Scan(&password)
		fmt.Println("\033[28m")
		user := database.GetUserByID("1")

		if utils.PasswordHash(password) == user.Password {
			hostInfo := models.HostInfo{
				Name:        host.Info().Name,
				Description: host.Info().Platform + "/" + host.Info().Arch,
				IP:          utils.GetLocalIP(),
				Port:        utils.GetLocalPort(),
				Key:         database.GetApiKey(),
			}

			hostInfoJSON, err := json.Marshal(hostInfo)
			if err != nil {
				fmt.Println(err)
			}

			config := qrterminal.Config{
				Level:     qrterminal.M,
				Writer:    os.Stdout,
				BlackChar: qrterminal.WHITE,
				WhiteChar: qrterminal.BLACK,
				QuietZone: 2,
			}
			qrterminal.GenerateWithConfig(string(hostInfoJSON), config)

		} else {
			fmt.Println("Wrong password.")
		}
	},
}
