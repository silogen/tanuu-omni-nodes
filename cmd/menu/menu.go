package menu

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	log "github.com/sirupsen/logrus"

	"github.com/tanuudev/tanuu-omni-nodes/cmd/create"
	"github.com/tanuudev/tanuu-omni-nodes/cmd/utils"
)

// https://github.com/charmbracelet/huh/blob/main/examples/burger/main.go

type environment struct {
	Name string
}

// Menu is the main function for handling the menu.
func Menu() {
	environment := environment{}

	log.Info("starting up the menu...")
	// Should we run in accessible mode?
	accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))
	action := ""
	form := huh.NewForm(
		// huh.NewGroup(huh.NewNote().
		// 	Title("TanuuDev").
		// 	Description("Welcome to TanuuDev.\n\nLets get started")),

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
					Description("environment should we create?.").
					Placeholder("test"),
				// TODO add some validation here
			),
		).WithAccessible(accessible)
		err := form.Run()

		if err != nil {
			log.Fatal("Uh oh:", err)
		}

		suffix, err := utils.GenerateRandomString(5)
		if err != nil {
			log.Error("Error generating random string: ", err)
		}
		envname := environment.Name + "-" + suffix

		createenv := func() {
			create.Createenvironment(envname)
		}
		//spinner while running func
		log.Debug("Creating environment with name: ", envname)
		_ = spinner.New().Title("Preparing your environment...").Accessible(accessible).Action(createenv).Run()

		// Print order summary.
		{
			var sb strings.Builder
			fmt.Fprintf(&sb,
				"%s",
				lipgloss.NewStyle().Bold(true).Render("Environment "+envname+" Created."),
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
		log.Debug("Deleting environment with name: ", environment.Name)
		utils.DeleteOmniCluster(environment.Name)
		nodes, err := utils.FindReadyNodes(environment.Name)
		if err != nil {
			log.Fatal("Error finding nodes: ", err)
		}
		for _, node := range nodes {
			log.Debug("Node: ", node.Metadata.ID, " Hostname: ", node.Spec.Platformmetadata.Hostname)
			utils.DeleteOmniMachine(node.Metadata.ID)
		}
		log.Debug("Machines and Cluster deleted")
		utils.DeleteNodes(environment.Name)

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
