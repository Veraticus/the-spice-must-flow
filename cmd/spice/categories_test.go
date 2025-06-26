package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestAddCategoryCmd(t *testing.T) {
	cmd := addCategoryCmd()

	// Test that the confidence-threshold flag exists with default value
	flag := cmd.Flag("confidence-threshold")
	assert.NotNil(t, flag, "confidence-threshold flag should exist")
	assert.Equal(t, "0.95", flag.DefValue, "default confidence threshold should be 0.95")

	// Test that the flag description is correct
	assert.Contains(t, flag.Usage, "Prompt for description when AI confidence is below this threshold")
}

func TestCategoriesCmd(t *testing.T) {
	cmd := categoriesCmd()

	// Test that subcommands exist
	assert.NotNil(t, cmd)

	// Find the add subcommand
	var addCmd *cobra.Command
	for _, subcmd := range cmd.Commands() {
		if subcmd.Name() == "add" {
			addCmd = subcmd
			break
		}
	}

	assert.NotNil(t, addCmd, "add subcommand should exist")
}
