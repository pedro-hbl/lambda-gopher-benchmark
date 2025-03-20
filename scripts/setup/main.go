package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	dynamoDBSetupScript   = "/scripts/dynamodb.sh"
	immuDBSetupScript     = "/scripts/immudb.sh"
	timestreamSetupScript = "/scripts/timestream.sh"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)

	// Get the database to set up from command line args
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"all"}
	}

	// Process each database setup request
	for _, db := range args {
		db = strings.ToLower(db)
		switch db {
		case "all":
			setupDynamoDB()
			// These will be enabled when implemented
			// setupImmuDB()
			// setupTimestream()
			return
		case "dynamodb":
			setupDynamoDB()
		case "immudb":
			setupImmuDB()
		case "timestream":
			setupTimestream()
		default:
			log.Fatalf("Unknown database type: %s", db)
		}
	}
}

func setupDynamoDB() {
	log.Println("Setting up DynamoDB...")
	runScript(dynamoDBSetupScript)
}

func setupImmuDB() {
	log.Println("Setting up ImmuDB...")
	log.Println("ImmuDB setup is not yet implemented.")
	// When implemented:
	// runScript(immuDBSetupScript)
}

func setupTimestream() {
	log.Println("Setting up AWS Timestream...")
	log.Println("Timestream setup is not yet implemented.")
	// When implemented:
	// runScript(timestreamSetupScript)
}

func runScript(scriptPath string) {
	// If script path is relative, convert to absolute
	if !strings.HasPrefix(scriptPath, "/") {
		scriptPath = filepath.Join("/scripts", scriptPath)
	}

	// Check if script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Fatalf("Setup script not found: %s", scriptPath)
	}

	// Make script executable
	if err := os.Chmod(scriptPath, 0755); err != nil {
		log.Fatalf("Failed to make script executable: %v", err)
	}

	// Run the script
	cmd := exec.Command(scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running script: %s", scriptPath)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to run setup script: %v", err)
	}

	log.Printf("Script %s completed successfully", scriptPath)
}
