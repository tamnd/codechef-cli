package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) problemsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "problems",
		Short: "List CodeChef problems",
		Long:  `Fetch the most recent CodeChef problems from the public problem list API.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching %d problems...", n)
			problems, err := a.client.Problems(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(problems, len(problems))
		},
	}
	return cmd
}
