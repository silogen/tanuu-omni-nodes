package create

import (
	"bytes"
	"context"
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
var claimbasetemp *template.Template
var claimgputemp *template.Template
var clustertemp *template.Template
var kubeconfigtemp *template.Template

// Using the init function to make sure the template is only parsed once in the program
func init() {
	// template.Must takes the reponse of template.ParseFiles and does ertemplror checking
	claimbasetemp = template.Must(template.ParseFS(tplFolder, "templates/claim-base.tmpl"))
	claimgputemp = template.Must(template.ParseFS(tplFolder, "templates/claim-gpu.tmpl"))
	clustertemp = template.Must(template.ParseFS(tplFolder, "templates/cluster.tmpl"))
	kubeconfigtemp = template.Must(template.ParseFS(tplFolder, "templates/kubeconfig.tmpl"))
}

// Createenvironment creates an environment
func Createenvironment(ctx context.Context, environment utils.Environment) error {
	log.Info("Creating environment with name: ", environment.Name)
	environment.TailScaleClientID = os.Getenv("TAILSCALE_CLIENT_ID")
	environment.TailScaleClientSecret = os.Getenv("TAILSCALE_CLIENT_SECRET")
	// Execute the template with the environment struct
	claimfile, err := os.OpenFile(environment.Name+"-composition.yaml", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening claimfile: %v", err)
		return err
	}
	defer claimfile.Close()
	clusterfile, err := os.OpenFile(environment.Name+"-cluster.yaml", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening clusterfile: %v", err)
		return err
	}
	defer clusterfile.Close()
	kubeconfigfile, err := os.OpenFile(environment.Name+".kubeconfig", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening kubeconfigfile: %v", err)
		return err
	}
	defer kubeconfigfile.Close()
	if environment.Gpu {
		err = claimgputemp.Execute(claimfile, environment)
		if err != nil {
			log.Fatalf("Error executing template: %v", err)
			return err
		}
	} else {
		err = claimbasetemp.Execute(claimfile, environment)
		if err != nil {
			log.Fatalf("Error executing template: %v", err)
			return err
		}
	}
	// check that fine kubeconfig exists
	if _, err := os.Stat("kubeconfig"); os.IsNotExist(err) {
		log.Fatalf("kubeconfig file does not exist: %v", err)
		return err
	}
	// commandstring := "KUBECONFIG=kubeconfig kubectl apply -f " + environment.Name + "-composition.yaml"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // Set your desired timeout
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", environment.Name+"-composition.yaml")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// cmd.Stdout = os.Stdout
	err = cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		log.Fatalf("Command timed out: %v", ctx.Err())
		return ctx.Err()
	}

	if err != nil {
		log.Fatalf("Error creating environment: %v, stderr: %s", err, stderr.String())
		return err
	}
	utils.WaitForReady(environment.Name)
	log.Debug("nodes are ready")
	// get the omni node ID's
	time.Sleep(30 * time.Second)
	err = nil
	nodes, err := utils.FindReadyNodes(environment.Name)
	if err != nil {
		log.Fatalf("Error finding ready nodes: %v", err)
		return err
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
		log.Fatalf("Error executing template: %v", err)
		return err
	}
	// apply the omni template
	utils.ApplyCluster(environment)
	time.Sleep(30 * time.Second)
	err = kubeconfigtemp.Execute(kubeconfigfile, environment)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
		return err
	}
	utils.WaitForCluster(environment)
	os.Remove(environment.Name + "-composition.yaml")
	os.Remove(environment.Name + "-cluster.yaml")
	return nil
}
