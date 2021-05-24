// +build ignore

package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

type versionInfo struct {
	Major string
	Minor string
	Patch string
	Year  int
}

func main() {

	versionCmd := flag.String("version", "0.0.0", `go run releaser.go -version="1.0.0" `)

	flag.Parse()
	versionArr := strings.Split(*versionCmd, ".")

	if len(versionArr) < 3 {
		log.Fatal("Incorrect version format provided")
	}

	year := time.Now().Year()

	version := versionInfo{
		Major: versionArr[0],
		Minor: versionArr[1],
		Patch: versionArr[2],
		Year:  year,
	}

	versionTemplateFile, err := ioutil.ReadFile("versioninfo.json.template")
	if err != nil {
		log.Fatal(err)
	}

	//Version Info file
	versionInfoFileString := string(versionTemplateFile)

	versionTemplate := template.New("Version")

	versionTemplate, _ = versionTemplate.Parse(versionInfoFileString)

	versionFile, err := os.Create("versioninfo.json")
	if err != nil {
		log.Fatal(err)
	}

	err = versionTemplate.Execute(versionFile, version)
	if err != nil {
		fmt.Println(err)
	}

	versionFile.Close()

	//Release SVG shield for README.md
	releaseSVGTemplateFile, err := ioutil.ReadFile("release.svg.template")
	if err != nil {
		log.Fatal(err)
	}

	releaseSVGString := string(releaseSVGTemplateFile)

	svgTemplate := template.New("SVG")

	svgTemplate, _ = svgTemplate.Parse(releaseSVGString)

	releaseFile, err := os.Create("images/release.svg")

	if err != nil {
		log.Fatal(err)
	}

	err = svgTemplate.Execute(releaseFile, version)
	if err != nil {
		fmt.Println(err)
	}

	releaseFile.Close()

	//App Version Package
	versionGoTemplateFile, err := ioutil.ReadFile("version.go.template")
	if err != nil {
		log.Fatal(err)
	}

	versionGoString := string(versionGoTemplateFile)

	versionGoTemplate := template.New("versionGo")

	versionGoTemplate, _ = versionGoTemplate.Parse(versionGoString)

	versionGoFile, err := os.Create("version/version.go")

	if err != nil {
		log.Fatal(err)
	}

	err = versionGoTemplate.Execute(versionGoFile, version)
	if err != nil {
		fmt.Println(err)
	}

	releaseFile.Close()

	fmt.Println("done")

}
