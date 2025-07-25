package main

import (
	"context"
	"fmt"
	"os"

	"github.com/polatengin/vermont/internal/config"
	"github.com/polatengin/vermont/internal/logger"
	"github.com/polatengin/vermont/pkg/executor"
	"github.com/polatengin/vermont/pkg/workflow"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vermont",
		Short: "Vermont - Lightweight GitHub Actions Runner Clone",
		Long: `Vermont is a lightweight, self-hosted GitHub Actions runner clone written in Go.
It executes YAML workflows with support for basic GitHub Actions features.`,
		Version:       fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		SilenceErrors: true, // We handle errors in main()
	}

	// Add subcommands
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newValidateCommand())

	return cmd
}

func newRunCommand() *cobra.Command {
	var configFile string
	var verbose bool

	cmd := &cobra.Command{
		Use:          "run [workflow.yml]",
		Short:        "Run a workflow file",
		Long:         "Execute a GitHub Actions workflow file",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true, // Don't show usage on errors
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflow(args[0], configFile, verbose)
		},
	}

	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Configuration file path")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	return cmd
}

func newValidateCommand() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:          "validate [workflow.yml]",
		Short:        "Validate a workflow file",
		Long:         "Validate the syntax and structure of a GitHub Actions workflow file",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true, // Don't show usage on errors
		RunE: func(cmd *cobra.Command, args []string) error {
			return validateWorkflow(args[0], verbose)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	return cmd
}

func runWorkflow(workflowFile, configFile string, verbose bool) error {
	// Initialize logger
	log := logger.New(verbose)
	log.Info("Starting Vermont Runner",
		"version", version,
		"commit", commit,
		"date", date,
	)

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create workflow parser
	parser := workflow.NewParser(log)

	// Create executor
	exec := executor.New(cfg, log)

	// Execute workflow
	log.Info("Executing workflow", "file", workflowFile)

	wf, err := parser.ParseFile(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	ctx := context.Background()
	if err := exec.Execute(ctx, wf); err != nil {
		return fmt.Errorf("failed to execute workflow: %w", err)
	}

	log.Info("Workflow execution completed successfully")
	return nil
}

func validateWorkflow(filename string, verbose bool) error {
	log := logger.New(verbose)
	parser := workflow.NewParser(log)

	log.Info("Validating workflow file", "file", filename)

	wf, err := parser.ParseFile(filename)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Always show validation success (not subject to verbose flag)
	fmt.Printf("✅ Workflow is valid: %s (%d jobs)\n", wf.Name, len(wf.Jobs))

	if verbose {
		for jobID, job := range wf.Jobs {
			log.Info("Job details", "id", jobID, "steps", len(job.Steps))
		}
	}

	return nil
}
