package utils

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

func ConfigFileSetup() {
	if _, err := os.Stat("config.ini"); err != nil {
		configFile, err := os.OpenFile("config.ini", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		LogError(err)
		defer configFile.Close()

		defaultConfigOpts := []string{
			"port: 8000",
			"open_browser_on_startup: true",
			"save_logs: false",
			"ssl_enabled: false",
			"# ssl_certificate: /path/to/file.crt ",
			"# ssl_certificate_key: /path/to/file.key",
		}

		defaultConfig := strings.Join(defaultConfigOpts, "\n")
		_, err = configFile.WriteString(defaultConfig)
		LogError(err)
	}

	runPath, err := os.Getwd()
	LogError(err)

	viper.SetConfigName("config.ini")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(runPath)
	err = viper.ReadInConfig()
	if err != nil {
		LogError(err)
	}
}

//OpenBrowser opens default web browser in specific domain
func OpenBrowser(domain, port string) {
	url := domain + ":" + port
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

//RandString returns random string with specific length
func RandString(length int) string {
	return stringWithCharset(length, charset)
}

//StartMessage displays message on server start up
func StartMessage(protocol, port string) {
	fmt.Printf(`       
     _____________
    /            /\                 
   /   /    /   / /   ___   (_)/ /_ ____
  /   /    /   / /   / _ \ / // __// __/    
 /            / /   /_//_//_/ \__//_/
/____________/ / 	    
\____________\/     v0.4.0

Go to admin panel at %v://localhost:%v

`, protocol, port)
}

func LogError(e error) {
	if e != nil {
		log.Println(e)
	}
}
