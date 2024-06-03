package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/tanuudev/tanuu-omni-nodes/cmd/create"
	"github.com/tanuudev/tanuu-omni-nodes/cmd/utils"
)

var rootCmd = &cobra.Command{
	Use:   "tannu-omni",
	Short: "App to create environments",
	Long:  `This application creates environments for you to work in.`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(createCmd)
}

var name string
var gpu bool

// helloCmd represents the hello command
var createCmd = &cobra.Command{
	Use:   "create [message]",
	Short: "create an environment",
	Long:  `Create an environment.`,
	Run: func(cmd *cobra.Command, args []string) {
		match, _ := regexp.MatchString("^[a-z0-9-]+$", name)
		if !match {
			log.Fatalf("Error: name must only contain lowercase letters, numbers, and dashes")
		}
		environment := utils.Environment{}
		suffix, err := utils.GenerateRandomString(5)
		if err != nil {
			log.Error("Error generating random string: ", err)
		}
		environment.Name = name + "-" + suffix
		environment.Gpu = gpu
		log.Info("Creating environment with name: ", environment.Name)
		createenv := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // Set your desired timeout
			defer cancel()
			err := create.Createenvironment(ctx, environment)
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					log.Fatalf("Command timed out: %v", ctx.Err())
				} else {
					log.Fatalf("Error creating environment: %v", err)
				}
			}
		}
		createenv()

	},
}

func init() {
	createCmd.Flags().StringVarP(&name, "name", "n", "", "Name of the environment to create")
	createCmd.MarkFlagRequired("name")
	createCmd.Flags().BoolVarP(&gpu, "gpu", "g", false, "Enable GPU for the environment")

}
