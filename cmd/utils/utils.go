package utils

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Error("Error creating new request: ", err)
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error executing GET request: ", err)
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		log.Error("Error creating file: ", err)
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Error("Error writing to file: ", err)
	}

	return err
}

// WaitForReady waits for the managed nodes to be ready
func WaitForReady() {
	log.Debug("Waiting for the managed nodes to be ready")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Error("Timeout waiting for the managed nodes to be ready")
			return
		default:
			cmd := exec.Command("kubectl", "get", "managed", "-o", "jsonpath='{$.items[*].status.conditions[?(@.type==\"Ready\")].status}'")
			output, err := cmd.Output()
			log.Debug(cmd.String())
			log.Debug("Output: ", string(output))
			if err != nil {
				log.Error("Error executing kubectl command: ", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if !strings.Contains(string(output), "False") {
				return
			}
			log.Debug("Nodes not ready")
			time.Sleep(5 * time.Second)
		}
	}
}

// WaitForCluster waits for the managed cluster to be ready
func WaitForCluster(environment Environment) {
	log.Debug("Waiting for the managed cluster to be ready")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Error("Timeout waiting for the managed cluster to be ready")
			return
		default:
			cmd := exec.Command("omnictl", "cluster", "status", environment.Name)
			log.Debug("Command: ", cmd)
			output, err := cmd.Output()
			if err != nil {
				log.Error("Error executing command: ", err)
				time.Sleep(5 * time.Second)
				continue
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
}

// FindReadyNodes finds the ready nodes
func FindReadyNodes(environment string) ([]Machine, error) {
	var machines Machines
	nodelist := &bytes.Buffer{}
	cmd := exec.Command("omnictl", "get", "machines", "-o", "jsonpath='{.metadata.id}'")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nodelist

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run the command
	err := cmd.Run()
	if err != nil {
		log.Error("Error executing command: ", err)
		return nil, err
	}

	log.Debug("Node list: ", nodelist.String())
	nodestring := strings.Trim(nodelist.String(), "'\n")
	nodeids := strings.Split(nodestring, "\n'\n'\n")
	for _, nodeid := range nodeids {
		// Trim leading and trailing newline and quote characters from each node ID
		nodeid = strings.Trim(nodeid, "'\n")
		log.Debug("Node ID: ", nodeid)
		cmd := exec.CommandContext(ctx, "omnictl", "get", "machinestatus", nodeid, "-o", "json")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		// Run the command
		err := cmd.Run()
		if err != nil {
			log.Error("Error running command: ", err)
			return nil, err
		}

		node := Machine{}
		err = json.Unmarshal(stdout.Bytes(), &node)
		if err != nil {
			log.Error("Error unmarshalling JSON: ", err)
			return nil, err
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Set your desired timeout
	defer cancel()

	cmd := exec.CommandContext(ctx, "omnictl", "cluster", "template", "sync", "-f", environment.Name+"-cluster.yaml")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	log.Println("Applying cluster: ", cmd)
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		log.Fatalf("Command timed out: %v", ctx.Err())
		return
	}

	if err != nil {
		log.Fatalf("Error creating environment: %v, stderr: %s", err, stderr.String())
	}
}

// ListClusters lists the clusters
func ListClusters() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Set your desired timeout
	defer cancel()

	clusters := &bytes.Buffer{}
	clusterlist := []string{}
	cmd := exec.CommandContext(ctx, "omnictl", "get", "clusters", "-o", "jsonpath='{.metadata.id}'")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = clusters
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		log.Fatalf("Command timed out: %v", ctx.Err())
		return nil, ctx.Err()
	}

	if err != nil {
		log.Fatalf("Error finding environment: %v, stderr: %s", err, stderr.String())
		return nil, err
	}

	log.Debug("Cluster list: ", clusters.String())
	clusterstring := strings.Trim(clusters.String(), "'\n")
	clusterList := strings.Split(clusterstring, "\n'\n'\n")

	for _, clusterid := range clusterList {
		// Trim leading and trailing newline and quote characters from each node ID
		clusterid = strings.Trim(clusterid, "'\n")
		log.Debug("Cluster ID: ", clusterid)
		clusterlist = append(clusterlist, clusterid)
	}
	return clusterlist, nil
}

// DeleteOmniCluster deletes the cluster
func DeleteOmniCluster(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Set your desired timeout
	defer cancel()

	cmd := exec.CommandContext(ctx, "omnictl", "cluster", "delete", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		log.Fatalf("Command timed out: %v", ctx.Err())
		return
	}

	if err != nil {
		log.Fatalf("Error deleting environment: %v, stderr: %s", err, stderr.String())
		return
	}

	log.Debug("Cluster deleted: ", name)
}

// DeleteOmniMachine deletes the machine
func DeleteOmniMachine(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Set your desired timeout
	defer cancel()

	cmd := exec.CommandContext(ctx, "omnictl", "delete", "link", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		log.Fatalf("Command timed out: %v", ctx.Err())
		return
	}

	if err != nil {
		log.Fatalf("Error deleting machine: %v, stderr: %s", err, stderr.String())
		return
	}

	log.Debug("Machine deleted: ", name)
}

// DeleteNodes Deletes for the managed nodes
func DeleteNodes(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Set your desired timeout
	defer cancel()

	nodes := &bytes.Buffer{}
	log.Debug("Deleting node claims")

	cmd := exec.CommandContext(ctx, "kubectl", "get", "nodegroupclaims", "-o", "NAME", "--no-headers")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nodes
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		log.Fatalf("Command timed out: %v", ctx.Err())
		return
	}

	if err != nil {
		log.Fatalf("Error getting nodegroupclaims: %v, stderr: %s", err, stderr.String())
		return
	}

	for _, node := range strings.Split(nodes.String(), "\n") {
		if strings.Contains(node, name) {
			log.Debug("Deleting nodegroupclaim: ", node)
			cmd := exec.CommandContext(ctx, "kubectl", "delete", node)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()

			if ctx.Err() == context.DeadlineExceeded {
				log.Fatalf("Command timed out: %v", ctx.Err())
				return
			}

			if err != nil {
				log.Fatalf("Error deleting nodegroupclaim: %v, stderr: %s", err, stderr.String())
				return
			}
		}
	}
}
