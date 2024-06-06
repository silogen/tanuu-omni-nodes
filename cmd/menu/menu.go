package menu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	log "github.com/sirupsen/logrus"

	"github.com/tanuudev/tanuu-omni-nodes/cmd/create"
	"github.com/tanuudev/tanuu-omni-nodes/cmd/utils"
)

// https://github.com/charmbracelet/huh/blob/main/examples/burger/main.go

// Menu is the main function for handling the menu.
func Menu() {
	environment := utils.Environment{}

	log.Info("starting up the menu...")
	// Should we run in accessible mode?
	// accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))
	accessible := true
	action := ""
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Options(huh.NewOptions("Create Environment", "Delete Environment")...).
				Title("Choose your action").
				Description("We can create, or delete, not much more.").
				Value(&action),
		),
	).WithAccessible(accessible)

	err := form.Run()

	if err != nil {
		log.Fatal("Uh oh:", err)
	}
	if action == "Create Environment" {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Value(&environment.Name).
					Title("Choose your environment name (will be appended with random characters)").
					Description("environment should we create?.\ntype exit to just exit.").
					Placeholder("test").
					Validate(func(val string) error { // Modify the function signature to take a string argument
						match, _ := regexp.MatchString("^[a-z0-9-]+$", val)
						if !match {
							return errors.New("input must only contain lowercase letters, numbers, and dashes")
						}

						return nil
					}),
				huh.NewConfirm().
					Title("Does this need GPU's?").
					Value(&environment.Gpu).
					Affirmative("Yes!").
					Negative("No."),
			),
		).WithAccessible(accessible)
		err := form.Run()

		if err != nil {
			log.Fatal("Uh oh:", err)
		}
		log.Debug("Creating environments: ", environment.Name)
		log.Debug("Gpu: ", environment.Gpu)
		if environment.Name == "exit" {
			log.Info("Exiting...")
			os.Exit(0)
		}
		suffix, err := utils.GenerateRandomString(5)
		if err != nil {
			log.Error("Error generating random string: ", err)
		}
		environment.Name = environment.Name + "-" + suffix

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
		//spinner while running func
		log.Debug("Creating environment with name: ", environment.Name)
		_ = spinner.New().Title("Preparing your environment...").Accessible(accessible).Action(createenv).Run()

		// Print order summary.
		{
			var sb strings.Builder
			fmt.Fprintf(&sb,
				"%s",
				lipgloss.NewStyle().Bold(true).Render("Environment "+environment.Name+" Created."),
			)

			fmt.Println(
				lipgloss.NewStyle().
					Width(40).
					BorderStyle(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("63")).
					Padding(1, 2).
					Render(sb.String()),
			)
		}
	} else if action == "Delete Environment" {

		log.Debug("Deleting environments")
		clusters, err := utils.ListClusters()
		if err != nil {
			log.Fatal("Error getting clusters: ", err)
		}
		clusters = append(clusters, "EXIT")
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Options(huh.NewOptions(clusters...)...).
					Title("Choose your environment to delete").
					Description("environment should we delete?").
					Value(&environment.Name),
			),
		).WithAccessible(accessible)
		err = form.Run()
		if err != nil {
			log.Fatal("Uh oh:", err)
		}
		if environment.Name == "EXIT" {
			log.Info("Exiting...")
			os.Exit(0)
		}
		log.Debug("Deleting environment with name: ", environment.Name)
		utils.DeleteOmniCluster(environment.Name)
		nodes, err := utils.FindReadyNodes(environment.Name)
		if err != nil {
			log.Error("Error finding nodes: ", err)
		} else {
			for _, node := range nodes {
				log.Debug("Node: ", node.Metadata.ID, " Hostname: ", node.Spec.Platformmetadata.Hostname)
				utils.DeleteOmniMachine(node.Metadata.ID)
			}
		}
		utils.DeleteNodes(environment.Name)
		log.Debug("Machines and Cluster deleted")
		files := []string{
			environment.Name + "-composition.yaml",
			environment.Name + "-cluster.yaml",
			environment.Name + ".kubeconfig",
		}

		for _, file := range files {
			if err := os.Remove(file); err != nil {
				if !os.IsNotExist(err) {
					log.Warnf("Failed to remove file: %v", err)
				}
			}
		}
		var sb strings.Builder
		log.Debug("Environment Deletion Completed.")
		fmt.Fprintf(&sb,
			"%s",
			lipgloss.NewStyle().Bold(true).Render("Environment Deletion Completed."),
		)

		fmt.Println(
			lipgloss.NewStyle().
				Width(40).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(1, 2).
				Render(sb.String()),
		)

	}
}
