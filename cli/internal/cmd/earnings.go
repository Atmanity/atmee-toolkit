package cmd

import "github.com/spf13/cobra"

func (a *app) earningsCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "earnings",
		Short: "The creator wallet: what monetized avatars earned and what was paid out.",
		Long: "What the account's monetized avatars earned, what was paid out, and the wallet balance between the two. This is not the " +
			"credit balance sessions spend (`atmee balance`); one avatar's earnings are in `atmee avatars monetization`. " +
			"Works for a signed-in user, or an account-wide key with avatars:read.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, err := a.api()
			if err != nil {
				return err
			}
			e, err := cl.Earnings(ctx())
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(e)
			}
			a.printf("Wallet       %s credits\n", credits(e.WalletBalanceMicrocredits))
			a.printf("Earned       %s credits all time\n", credits(e.TotalEarningsMicrocredits))
			a.printf("Paid out     %s credits\n", credits(e.TotalPayoutsMicrocredits))
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}
