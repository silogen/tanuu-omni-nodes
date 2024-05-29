package create

import (
	"bytes"
	"embed"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/tanuudev/tanuu-omni-nodes/cmd/utils"
)

//go:embed templates/*
var tplFolder embed.FS

// Declare type pointer to a template
var claimtemp *template.Template
var clustertemp *template.Template
var kubeconfigtemp *template.Template

// Using the init function to make sure the template is only parsed once in the program
func init() {
	// template.Must takes the reponse of template.ParseFiles and does ertemplror checking
	claimtemp = template.Must(template.ParseFS(tplFolder, "templates/claim.tmpl"))
	clustertemp = template.Must(template.ParseFS(tplFolder, "templates/cluster.tmpl"))
	kubeconfigtemp = template.Must(template.ParseFS(tplFolder, "templates/kubeconfig.tmpl"))
}

// Createenvironment creates an environment
func Createenvironment(name string) error {
	log.Info("Creating environment with name: ", name)
	environment := utils.Environment{}
	environment.Name = name
	environment.TailScaleClientID = os.Getenv("TAILSCALE_CLIENT_ID")
	environment.TailScaleClientSecret = os.Getenv("TAILSCALE_CLIENT_SECRET")
	// Execute the template with the environment struct
	claimfile, err := os.OpenFile(environment.Name+"-composition.yaml", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	defer claimfile.Close()
	clusterfile, err := os.OpenFile(environment.Name+"-cluster.yaml", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	defer clusterfile.Close()
	kubeconfigfile, err := os.OpenFile(environment.Name+".kubeconfig", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	defer kubeconfigfile.Close()
	err = claimtemp.Execute(claimfile, environment)
	if err != nil {
		log.Fatal("Error executing template: ", err)
		return err
	}
	// check that fine kubeconfig exists
	if _, err := os.Stat("kubeconfig"); os.IsNotExist(err) {
		log.Fatal("kubeconfig file does not exist")
	}
	// commandstring := "KUBECONFIG=kubeconfig kubectl apply -f " + environment.Name + "-composition.yaml"
	cmd := exec.Command("kubectl", "apply", "-f", environment.Name+"-composition.yaml")
	cmd.Env = append(os.Environ(), "KUBECONFIG=kubeconfig")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		log.Fatal("Error creating environment: ", err)
		log.Fatal("Error creating environment: ", stderr.String())
		return err
	}
	utils.WaitForReady()
	// get the omni node ID's
	time.Sleep(30 * time.Second)
	nodes, err := utils.FindReadyNodes(environment.Name)
	if err != nil {
		log.Fatal("Error finding ready nodes: ", err)
	}
	log.Debug("Nodes: ", nodes)
	gpulist := []string{}
	workerlist := []string{}
	ctlrlist := []string{}
	for _, node := range nodes {
		log.Debug("Node: ", node.Metadata.ID, " Hostname: ", node.Spec.Platformmetadata.Hostname)
		if strings.Contains(node.Spec.Platformmetadata.Hostname, "gpu") {
			gpulist = append(gpulist, "  - "+node.Metadata.ID)
		} else if strings.Contains(node.Spec.Platformmetadata.Hostname, "worker") {
			workerlist = append(workerlist, "  - "+node.Metadata.ID)
		} else if strings.Contains(node.Spec.Platformmetadata.Hostname, "ctlr") {
			ctlrlist = append(ctlrlist, "  - "+node.Metadata.ID)
		}
	}

	environment.ControlPlane = strings.Join(ctlrlist, "\n")
	environment.Workers = strings.Join(workerlist, "\n")
	environment.Gpus = strings.Join(gpulist, "\n")
	environment.GitHubToken = os.Getenv("GITHUB_TOKEN")
	log.Debug("Control Plane: ", environment.ControlPlane)
	log.Debug("Workers: ", environment.Workers)
	log.Debug("Gpus: ", environment.Gpus)
	err = clustertemp.Execute(clusterfile, environment)
	if err != nil {
		log.Fatal("Error executing template: ", err)
		return err
	}
	// apply the omni template
	utils.ApplyCluster(environment)
	time.Sleep(30 * time.Second)
	err = kubeconfigtemp.Execute(kubeconfigfile, environment)
	if err != nil {
		log.Fatal("Error executing template: ", err)
		return err
	}
	utils.WaitForCluster(environment)
	os.Remove(environment.Name + "-composition.yaml")
	os.Remove(environment.Name + "-cluster.yaml")
	return nil
}
