package cmd

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "CLI TOOL",
		Short: "A simple cloud backup tool",
	}
	//cmd.AddCommand()
	return cmd
}
