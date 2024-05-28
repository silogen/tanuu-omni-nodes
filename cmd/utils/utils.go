package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/json"
)

var (
	// OmniURL is the URL for the Omni API
	OmniURL string
	// OmniAuth is the authentication token for the Omni API
	OmniAuth string
)

// Environment struct to hold the environment variables
type Environment struct {
	Name                  string
	ControlPlane          string
	Workers               string
	Gpus                  string
	TailScaleClientID     string
	TailScaleClientSecret string
	GitHubToken           string
}

// Machines is the struct for the machines
type Machines struct {
	Machines []Machine `json:"machines"`
}

// Machine is the struct for the machine
type Machine struct {
	Metadata struct {
		ID     string            `json:"id"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Connected        bool `json:"connected"`
		Platformmetadata struct {
			Hostname     string `json:"hostname"`
			Instanceid   string `json:"instanceid"`
			Instancetype string `json:"instancetype"`
			Platform     string `json:"platform"`
			Providerid   string `json:"providerid"`
			Region       string `json:"region"`
		} `json:"platformmetadata"`
	} `json:"spec"`
}

// GenerateRandomString generates a random string
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length/2) // because 2 hex characters represent 1 byte
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Config is the struct for the an individual tool config
type Config struct {
	HelmChartName string `yaml:"helm-chart-name"`
	HelmURL       string `yaml:"helm-url"`
	Values        string `yaml:"values"`
	Secrets       bool   `yaml:"secrets"`
	Name          string `yaml:"name"`
	HelmName      string `yaml:"helm-name"`
	ManifestURL   string `yaml:"manifest-url"`
	Filename      string
}

// Setup sets up the logging
func Setup() {
	OmniURL := os.Getenv("OMNI_ENDPOINT")
	OmniAuth := os.Getenv("OMNI_SERVICE_ACCOUNT_KEY")
	// print an error if the environment variable is not set
	if OmniURL == "" {
		log.Fatal("OMNI_URL environment variable not set")
	}
	if OmniAuth == "" {
		log.Fatal("OMNI_AUTH environment variable not set")
	}
	// Get the log level from the environment variable
	logLevelStr := os.Getenv("LOG_LEVEL")
	if logLevelStr == "" {
		logLevelStr = "DEFAULT"
	}
	logLevel, err := log.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = log.InfoLevel
	}

	// Set the log level
	log.SetLevel(logLevel)

	// Set the output destination to a file
	logfilename := os.Getenv("LOG_NAME")
	if logfilename == "" {
		logfilename = "app.log"
	}
	logfilename = "logs/" + logfilename
	file, err := os.OpenFile(logfilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
}

func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// WaitForReady waits for the managed nodes to be ready
func WaitForReady() {
	log.Debug("Waiting for the managed nodes to be ready")
	for {
		cmd := exec.Command("kubectl", "get", "managed", "-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}")
		cmd.Env = append(os.Environ(), "KUBECONFIG=kubeconfig")
		output, err := cmd.Output()
		if err != nil {
			fmt.Println("Error executing kubectl command:", err)
			return
		}

		if !strings.Contains(string(output), "False") {
			break
		}

		time.Sleep(5 * time.Second)
	}
}

// WaitForCluster waits for the managed cluster to be ready
func WaitForCluster(environment Environment) {
	log.Debug("Waiting for the managed cluster to be ready")
	for {
		cmd := exec.Command("omnictl", "cluster", "status", environment.Name)
		cmd.Env = append(os.Environ(), "OMNI_SERVICE_ACCOUNT_KEY="+OmniAuth)
		cmd.Env = append(os.Environ(), "OMNI_ENDPOINT="+OmniURL)
		output, err := cmd.Output()
		if err != nil {
			fmt.Println("Error executing command:", err)
			return
		}
		lines := strings.Split(string(output), "\n")

		for _, line := range lines {
			log.Debug("Cluster Status: ", line)
			if strings.Contains(line, "Cluster") && strings.Contains(line, "RUNNING") && !strings.Contains(line, "Not") {
				// This line starts with "Cluster" and contains both "RUNNING" and "Ready"
				return
			}
		}

		time.Sleep(5 * time.Second)
	}
}

// FindReadyNodes finds the ready nodes
func FindReadyNodes(environment string) ([]Machine, error) {
	var machines Machines
	nodelist := &bytes.Buffer{}
	cmd := exec.Command("omnictl", "get", "machines", "-o", "jsonpath='{.metadata.id}'")
	cmd.Env = append(os.Environ(), "OMNI_SERVICE_ACCOUNT_KEY="+OmniAuth)
	cmd.Env = append(os.Environ(), "OMNI_ENDPOINT="+OmniURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nodelist
	// Run the command
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("Node list: ", nodelist.String())
	nodestring := strings.Trim(nodelist.String(), "'\n")
	nodeids := strings.Split(nodestring, "\n'\n'\n")
	for _, nodeid := range nodeids {
		// Trim leading and trailing newline and quote characters from each node ID
		nodeid = strings.Trim(nodeid, "'\n")
		log.Debug("Node ID: ", nodeid)
		cmd := exec.Command("omnictl", "get", "machinestatus", nodeid, "-o", "json")
		cmd.Env = append(os.Environ(), "OMNI_SERVICE_ACCOUNT_KEY="+OmniAuth)
		cmd.Env = append(os.Environ(), "OMNI_ENDPOINT="+OmniURL)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		// Run the command
		err := cmd.Run()
		if err != nil {
			log.Error("error running command: ", err)
			log.Fatal(err)
		}
		node := Machine{}
		err = json.Unmarshal(stdout.Bytes(), &node)
		if err != nil {
			log.Fatal(err)
		}
		// add nodes if the spec.platformmetadata.hostname contains the environment name
		if strings.Contains(node.Spec.Platformmetadata.Hostname, environment) {
			machines.Machines = append(machines.Machines, node)
		}
	}
	log.Debug("Machines: ", machines.Machines)
	return machines.Machines, nil
}

// ApplyCluster applies the cluster
func ApplyCluster(environment Environment) {
	cmd := exec.Command("omnictl", "cluster", "template", "sync", "-f", environment.Name+"-cluster.yaml")
	cmd.Env = append(os.Environ(), "OMNI_SERVICE_ACCOUNT_KEY="+OmniAuth)
	cmd.Env = append(os.Environ(), "OMNI_ENDPOINT="+OmniURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal("Error creating environment: ", err)
		log.Fatal("Error creating environment: ", stderr.String())
	}
}

// ListClusters applies the cluster
func ListClusters() ([]string, error) {
	clusters := &bytes.Buffer{}
	clusterlist := []string{}
	cmd := exec.Command("omnictl", "get", "clusters", "-o", "jsonpath='{.metadata.id}'")
	cmd.Env = append(os.Environ(), "OMNI_SERVICE_ACCOUNT_KEY="+OmniAuth)
	cmd.Env = append(os.Environ(), "OMNI_ENDPOINT="+OmniURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = clusters
	err := cmd.Run()
	log.Debug("Cluster list: ", clusters.String())
	clusterstring := strings.Trim(clusters.String(), "'\n")
	clusterList := strings.Split(clusterstring, "\n'\n'\n")

	if err != nil {
		log.Fatal("Error finding environment: ", err)
		log.Fatal("Error finding environment: ", stderr.String())
	}
	for _, clusterid := range clusterList {
		// Trim leading and trailing newline and quote characters from each node ID
		clusterid = strings.Trim(clusterid, "'\n")
		log.Debug("Cluster ID: ", clusterid)
		clusterlist = append(clusterlist, clusterid)
	}
	return clusterlist, nil
}

// DeleteOmniCluster applies the cluster
func DeleteOmniCluster(name string) {
	cmd := exec.Command("omnictl", "cluster", "delete", name)
	cmd.Env = append(os.Environ(), "OMNI_SERVICE_ACCOUNT_KEY="+OmniAuth)
	cmd.Env = append(os.Environ(), "OMNI_ENDPOINT="+OmniURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal("Error deleting environment: ", err)
		log.Fatal("Error deleting environment: ", stderr.String())
	}
	log.Debug("Cluster deleted: ", name)
}

// DeleteOmniMachine applies the cluster
func DeleteOmniMachine(name string) {
	cmd := exec.Command("omnictl", "delete", "link", name)
	cmd.Env = append(os.Environ(), "OMNI_SERVICE_ACCOUNT_KEY="+OmniAuth)
	cmd.Env = append(os.Environ(), "OMNI_ENDPOINT="+OmniURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal("Error deleting environment: ", err)
		log.Fatal("Error deleting environment: ", stderr.String())
	}
	log.Debug("Machine deleted: ", name)
}

// DeleteNodes Deletes for the managed nodes
func DeleteNodes(name string) {
	nodes := &bytes.Buffer{}
	log.Debug("Deleting node claims")

	cmd := exec.Command("kubectl", "get", "nodegroupclaims", "-o", "NAME", "--no-headers")
	cmd.Env = append(os.Environ(), "KUBECONFIG=kubeconfig")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nodes
	err := cmd.Run()
	if err != nil {
		log.Fatal("Error getting nodegroupclaims: ", err)
		log.Fatal("Error getting nodegroupclaims: ", stderr.String())
	}
	for _, node := range strings.Split(nodes.String(), "\n") {
		if strings.Contains(node, name) {
			log.Debug("Deleting nodegroupclaim: ", node)
			cmd := exec.Command("kubectl", "delete", node)
			cmd.Env = append(os.Environ(), "KUBECONFIG=kubeconfig")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err != nil {
				log.Fatal("Error deleting nodegroupclaim: ", err)
				log.Fatal("Error deleting nodegroupclaim: ", stderr.String())
			}
		}
	}

}
