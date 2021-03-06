package main

import (
	"os"

	"github.com/clems4ever/authelia/internal/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func buildAutheliaBinary() {
	cmd := utils.CommandWithStdout("go", "build", "-o", "../../"+OutputDir+"/authelia")
	cmd.Dir = "cmd/authelia"
	cmd.Env = append(os.Environ(),
		"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1")

	err := cmd.Run()

	if err != nil {
		panic(err)
	}
}

func buildFrontend() {
	// Install npm dependencies
	cmd := utils.CommandWithStdout("npm", "ci")
	cmd.Dir = "client"

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

	// Then build the frontend
	cmd = utils.CommandWithStdout("npm", "run", "build")
	cmd.Dir = "client"

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

	if err := os.Rename("client/build", OutputDir+"/public_html"); err != nil {
		log.Fatal(err)
	}
}

// Build build Authelia
func Build(cobraCmd *cobra.Command, args []string) {
	log.Info("Building Authelia...")

	Clean(cobraCmd, args)

	log.Debug("Creating `" + OutputDir + "` directory")
	err := os.MkdirAll(OutputDir, os.ModePerm)

	if err != nil {
		panic(err)
	}

	log.Debug("Building Authelia Go binary...")
	buildAutheliaBinary()

	log.Debug("Building Authelia frontend...")
	buildFrontend()
}
