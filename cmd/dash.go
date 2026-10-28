package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/menu"
)

func newDashCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dash",
		Short: "Manage dashboards (\"default\" is the dashboard served at /)",
	}
	cmd.AddCommand(
		newDashListCmd(a),
		newDashAddCmd(a),
		newDashUpdateCmd(a),
		newDashRemoveCmd(a),
		newDashSetPasswordCmd(a),
		newDashClearPasswordCmd(a),
	)
	return cmd
}

// withDashboardRef opens the database and resolves a slug or "default".
func (a *app) withDashboardRef(ctx context.Context, slug string, fn func(*menu.Service, menu.Dashboard) error) error {
	return a.withMenu(ctx, func(s *menu.Service) error {
		d, err := s.DashboardBySlug(ctx, slug)
		if err != nil {
			return fmt.Errorf("dashboard %q: %w", slug, err)
		}
		return fn(s, d)
	})
}

func newDashListCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List dashboards",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withMenu(cmd.Context(), func(s *menu.Service) error {
				list, err := s.ListDashboards(cmd.Context())
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				if asJSON {
					enc := json.NewEncoder(out)
					enc.SetIndent("", "  ")
					return enc.Encode(list)
				}
				for _, d := range list {
					path, slug := "/", menu.DefaultSlug
					if d.Slug != nil {
						slug = *d.Slug
						path = "/dash/" + slug
					}
					access := "public"
					if d.Protected {
						access = "protected"
					}
					title := d.Title
					if title == "" {
						title = a.cfg.Title
					}
					fmt.Fprintf(out, "[%d] %s  path=%s  title=%q  items=%d  %s\n", d.ID, slug, path, title, d.ItemCount, access)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the dashboards as JSON")
	return cmd
}

func newDashAddCmd(a *app) *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "add SLUG",
		Short: "Add a dashboard served at /dash/SLUG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withMenu(cmd.Context(), func(s *menu.Service) error {
				d, err := s.CreateDashboard(cmd.Context(), menu.DashboardInput{Slug: &args[0], Title: title})
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "added dashboard %d at /dash/%s\n", d.ID, *d.Slug)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "dashboard title")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func newDashUpdateCmd(a *app) *cobra.Command {
	var slug, title string
	cmd := &cobra.Command{
		Use:   "update SLUG",
		Short: "Change the title or slug of a dashboard (unset flags keep their value)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withDashboardRef(cmd.Context(), args[0], func(s *menu.Service, d menu.Dashboard) error {
				in := menu.DashboardInput{Slug: d.Slug, Title: d.Title}
				if cmd.Flags().Changed("slug") {
					in.Slug = &slug
				}
				if cmd.Flags().Changed("title") {
					in.Title = title
				}
				if _, err := s.UpdateDashboard(cmd.Context(), d.ID, in); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "updated dashboard %d\n", d.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&slug, "slug", "", "new slug")
	cmd.Flags().StringVar(&title, "title", "", "new title (empty on the default dashboard uses the configured title)")
	return cmd
}

func newDashRemoveCmd(a *app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "remove SLUG",
		Short: "Remove a dashboard (use --force when it still has items)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withDashboardRef(cmd.Context(), args[0], func(s *menu.Service, d menu.Dashboard) error {
				if err := s.DeleteDashboard(cmd.Context(), d.ID, force); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "removed dashboard %s\n", args[0])
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "also remove the dashboard's items")
	return cmd
}

func newDashSetPasswordCmd(a *app) *cobra.Command {
	var stdin bool
	cmd := &cobra.Command{
		Use:   "set-password SLUG",
		Short: "Protect a dashboard with a password (8 to 72 bytes) and revoke its viewer sessions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := readPassword(cmd, stdin)
			if err != nil {
				return err
			}
			if err := auth.ValidateDashboardPassword(password); err != nil {
				return err
			}
			return a.withDashboardPassword(cmd, args[0], func(mgr *auth.Manager, d menu.Dashboard) error {
				if err := mgr.SetDashboardPassword(cmd.Context(), d.ID, password); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "dashboard %s is now protected; its viewer sessions were revoked\n", args[0])
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read the password from stdin (for scripts)")
	return cmd
}

func newDashClearPasswordCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "clear-password SLUG",
		Short: "Make a dashboard public again and revoke its viewer sessions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.withDashboardPassword(cmd, args[0], func(mgr *auth.Manager, d menu.Dashboard) error {
				if err := mgr.ClearDashboardPassword(cmd.Context(), d.ID); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "dashboard %s is now public\n", args[0])
				return nil
			})
		},
	}
}

func (a *app) withDashboardPassword(cmd *cobra.Command, slug string, fn func(*auth.Manager, menu.Dashboard) error) error {
	db, err := a.openDB(cmd.Context())
	if err != nil {
		return err
	}
	defer db.Close()

	d, err := menu.NewService(db).DashboardBySlug(cmd.Context(), slug)
	if err != nil {
		return fmt.Errorf("dashboard %q: %w", slug, err)
	}
	mgr, err := auth.NewManager(db, auth.Options{})
	if err != nil {
		return err
	}
	return fn(mgr, d)
}
