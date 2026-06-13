package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) contestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contests",
		Short: "List CodeChef contests (ongoing, upcoming, past)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching contests...")
			contests, err := a.client.Contests(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(contests, len(contests))
		},
	}
	return cmd
}
