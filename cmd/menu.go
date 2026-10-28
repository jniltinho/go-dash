package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jniltinho/go-dash/internal/menu"
)

func (a *app) withMenu(ctx context.Context, fn func(*menu.Service) error) error {
	db, err := a.openDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(menu.NewService(db))
}

// withDashboard resolves the --dash flag (a slug or "default") and calls fn
// with the dashboard id.
func (a *app) withDashboard(cmd *cobra.Command, fn func(s *menu.Service, dashboardID int64) error) error {
	slug, err := cmd.Flags().GetString("dash")
	if err != nil {
		return err
	}
	return a.withMenu(cmd.Context(), func(s *menu.Service) error {
		d, err := s.DashboardBySlug(cmd.Context(), slug)
		if err != nil {
			return fmt.Errorf("dashboard %q: %w", slug, err)
		}
		return fn(s, d.ID)
	})
}

// cliValidateOptions forbids iframe items pointing at the configured public URL.
func (a *app) cliValidateOptions() menu.ValidateOptions {
	opts := menu.ValidateOptions{}
	if o := menu.Origin(a.cfg.PublicURL); o != "" {
		opts.ForbiddenIframeOrigins = []string{o}
	}
	return opts
}

func newMenuCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "menu",
		Short: "Manage menu items",
	}
	cmd.PersistentFlags().String("dash", menu.DefaultSlug, "dashboard slug (\"default\" for the dashboard served at /)")
	cmd.AddCommand(
		newMenuListCmd(a),
		newMenuAddCmd(a),
		newMenuUpdateCmd(a),
		newMenuRemoveCmd(a),
		newMenuExportCmd(a),
		newMenuImportCmd(a),
	)
	return cmd
}

func newMenuListCmd(a *app) *cobra.Command {
	var asTree, asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List menu items",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withDashboard(cmd, func(s *menu.Service, dashboardID int64) error {
				tree, err := s.Tree(cmd.Context(), dashboardID, false)
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				if asJSON {
					enc := json.NewEncoder(out)
					enc.SetIndent("", "  ")
					return enc.Encode(tree)
				}
				if len(tree) == 0 {
					fmt.Fprintln(out, "no menu items")
					return nil
				}
				printTree(out, tree, 0, asTree)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&asTree, "tree", true, "indent items by hierarchy level")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the tree as JSON")
	return cmd
}

func printTree(w io.Writer, nodes []*menu.Node, depth int, indent bool) {
	for _, n := range nodes {
		prefix := ""
		if indent {
			prefix = strings.Repeat("  ", depth)
		}
		url := "-"
		if n.URL != nil {
			url = *n.URL
		}
		state := ""
		if !n.Enabled {
			state = " (disabled)"
		}
		fmt.Fprintf(w, "%s[%d] %s  target=%s  placement=%s  url=%s%s\n", prefix, n.ID, n.Title, n.Target, n.Placement, url, state)
		printTree(w, n.Items, depth+1, indent)
	}
}

// itemFlags holds flags shared by add and update.
type itemFlags struct {
	title, url, target, placement, icon, description string
	parent                                           int64
	position                                         int
	disabled                                         bool
}

func (f *itemFlags) register(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.title, "title", "", "item title")
	fl.StringVar(&f.url, "url", "", "item URL (http/https); empty for a group")
	fl.StringVar(&f.target, "target", "iframe", "how the URL opens: iframe or blank")
	fl.StringVar(&f.placement, "placement", "both", "where the item is shown: menu, card or both")
	fl.StringVar(&f.icon, "icon", "", "icon CSS class, e.g. \"mdi mdi-chart-line\"")
	fl.StringVar(&f.description, "description", "", "tooltip text")
	fl.Int64Var(&f.parent, "parent", 0, "parent item ID (0 = top level)")
	fl.IntVar(&f.position, "position", 0, "order among siblings")
	fl.BoolVar(&f.disabled, "disabled", false, "create/keep the item disabled")
}

func (f *itemFlags) input() menu.Input {
	enabled := !f.disabled
	in := menu.Input{
		Title:       f.title,
		URL:         optional(f.url),
		Target:      menu.Target(f.target),
		Placement:   menu.Placement(f.placement),
		Icon:        optional(f.icon),
		Description: optional(f.description),
		Position:    f.position,
		Enabled:     &enabled,
	}
	if f.parent > 0 {
		in.ParentID = &f.parent
	}
	return in
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func newMenuAddCmd(a *app) *cobra.Command {
	var f itemFlags
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a menu item",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withDashboard(cmd, func(s *menu.Service, dashboardID int64) error {
				item, err := s.Create(cmd.Context(), dashboardID, f.input(), a.cliValidateOptions())
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), item.ID)
				return nil
			})
		},
	}
	f.register(cmd)
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func newMenuUpdateCmd(a *app) *cobra.Command {
	var f itemFlags
	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a menu item (unset flags keep their current value)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid ID %q", args[0])
			}
			return a.withDashboard(cmd, func(s *menu.Service, dashboardID int64) error {
				current, err := s.Get(cmd.Context(), dashboardID, id)
				if err != nil {
					return err
				}
				in := mergeUpdate(cmd, current, f)
				item, err := s.Update(cmd.Context(), dashboardID, id, in, a.cliValidateOptions())
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "updated item %d\n", item.ID)
				return nil
			})
		},
	}
	f.register(cmd)
	return cmd
}

// mergeUpdate starts from the current item and applies only changed flags.
func mergeUpdate(cmd *cobra.Command, cur menu.Item, f itemFlags) menu.Input {
	enabled := cur.Enabled
	in := menu.Input{
		ParentID:    cur.ParentID,
		Title:       cur.Title,
		URL:         cur.URL,
		Target:      cur.Target,
		Placement:   cur.Placement,
		Icon:        cur.Icon,
		Description: cur.Description,
		Position:    cur.Position,
		Enabled:     &enabled,
	}
	changed := cmd.Flags().Changed
	if changed("title") {
		in.Title = f.title
	}
	if changed("url") {
		in.URL = optional(f.url)
	}
	if changed("target") {
		in.Target = menu.Target(f.target)
	}
	if changed("placement") {
		in.Placement = menu.Placement(f.placement)
	}
	if changed("icon") {
		in.Icon = optional(f.icon)
	}
	if changed("description") {
		in.Description = optional(f.description)
	}
	if changed("position") {
		in.Position = f.position
	}
	if changed("disabled") {
		enabled = !f.disabled
	}
	if changed("parent") {
		in.ParentID = nil
		if f.parent > 0 {
			in.ParentID = &f.parent
		}
	}
	return in
}

func newMenuRemoveCmd(a *app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "remove ID",
		Short: "Remove a menu item (use --force to remove its children too)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid ID %q", args[0])
			}
			return a.withDashboard(cmd, func(s *menu.Service, dashboardID int64) error {
				if err := s.Delete(cmd.Context(), dashboardID, id, force); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "removed item %d\n", id)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "also remove child items")
	return cmd
}

func newMenuExportCmd(a *app) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the menu tree as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withDashboard(cmd, func(s *menu.Service, dashboardID int64) error {
				doc, err := s.Export(cmd.Context(), dashboardID)
				if err != nil {
					return err
				}
				w := cmd.OutOrStdout()
				if output != "" {
					file, err := os.Create(output)
					if err != nil {
						return fmt.Errorf("creating %q: %w", output, err)
					}
					defer file.Close()
					w = file
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(doc)
			})
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file instead of stdout")
	return cmd
}

func newMenuImportCmd(a *app) *cobra.Command {
	var replace bool
	cmd := &cobra.Command{
		Use:   "import FILE",
		Short: "Import a menu tree exported by `menu export`",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading %q: %w", args[0], err)
			}
			var doc menu.Document
			if err := json.Unmarshal(data, &doc); err != nil {
				return fmt.Errorf("parsing %q: %w", args[0], err)
			}
			mode := menu.ImportAppend
			if replace {
				mode = menu.ImportReplace
			}
			return a.withDashboard(cmd, func(s *menu.Service, dashboardID int64) error {
				n, err := s.Import(cmd.Context(), dashboardID, doc, mode, a.cliValidateOptions())
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "imported %d items (%s)\n", n, mode)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&replace, "replace", false, "delete all existing items of the dashboard before importing")
	return cmd
}
