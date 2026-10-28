package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/database"
)

func newMigrateCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending database migrations and exit",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, err := database.Open(cmd.Context(), a.cfg.DB)
			if err != nil {
				return err
			}
			defer db.Close()

			applied, err := database.Migrate(cmd.Context(), db)
			if err != nil {
				return err
			}
			if len(applied) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "database is up to date")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied migrations: %s\n", strings.Join(applied, ", "))
			return nil
		},
	}
}

func newBackupCmd(a *app) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Write a consistent copy of the database (safe while the server runs)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			db, err := a.openDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if err := database.Backup(cmd.Context(), db, out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backup written to %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "backup file path (must not exist)")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}

func newAdminCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage the web administration",
	}

	var stdin bool
	setPassword := &cobra.Command{
		Use:   "set-password",
		Short: "Set the admin password (stored as bcrypt) and revoke all sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			password, err := readPassword(cmd, stdin)
			if err != nil {
				return err
			}

			db, err := a.openDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			mgr, err := auth.NewManager(db, auth.Options{
				EnvPasswordHash: a.cfg.AdminPasswordHash,
				EnvPassword:     a.cfg.AdminPassword,
			})
			if err != nil {
				return err
			}
			if err := mgr.SetPassword(cmd.Context(), password); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "admin password updated; existing sessions were revoked")
			if mgr.EnvOverrides() {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"warning: DASHBOARD_ADMIN_PASSWORD(_HASH) is set and takes precedence; the stored password has no effect while it exists")
			}
			return nil
		},
	}
	setPassword.Flags().BoolVar(&stdin, "stdin", false, "read the password from stdin (for scripts)")

	cmd.AddCommand(setPassword)
	return cmd
}

func readPassword(cmd *cobra.Command, fromStdin bool) (string, error) {
	if fromStdin || !term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("reading password: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fmt.Fprint(cmd.ErrOrStderr(), "New password: ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Repeat password: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "go-dash %s (commit %s, built %s)\n", Version, GitCommit, BuildDate)
			return nil
		},
	}
}
