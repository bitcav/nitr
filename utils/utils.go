package utils

import (
	"fmt"
	"log"
	"math/rand"
	"os/exec"
	"runtime"
	"time"
)

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
	fmt.Printf(`                 _  __       
         ____   (_)/ /_ _____
   ____ / __ \ / // __// ___/
 _____ / / / // // /_ / /    
   __ /_/ /_//_/ \__//_/ v0.3.0    

Go to admin panel at %v://localhost:%v

`, protocol, port)
}
