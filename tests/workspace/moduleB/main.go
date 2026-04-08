package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"example.com/workspace/moduleA"
)

func main() {
	cmd := &cobra.Command{
		Use:          "moduleB",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(moduleA.Greet("workspace"))
			return nil
		},
	}
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
