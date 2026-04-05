package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type profile struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type profileStore struct {
	Profiles map[string]profile `json:"profiles"`
}

const defaultProfileName = "personal"

func main() {
	root := &cobra.Command{
		Use:   "gprof",
		Short: "Manage git user profiles",
	}

	root.AddCommand(newListCmd())
	root.AddCommand(newCurrentCmd())
	root.AddCommand(newUseCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newRemoveCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newListCmd() *cobra.Command {
	var detail bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadStore()
			if err != nil {
				return err
			}
			currentName := currentProfileName(store)
			names := make([]string, 0, len(store.Profiles))
			for name := range store.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				prof := store.Profiles[name]
				prefix := ""
				if name == currentName {
					prefix = "* "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", prefix, name)
				if detail {
					fmt.Fprintf(cmd.OutOrStdout(), "    name:%s\n", prof.Name)
					fmt.Fprintf(cmd.OutOrStdout(), "    email:%s\n", prof.Email)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&detail, "detail", "d", false, "Show profile details")

	return cmd
}

func newCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show current git profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := gitConfigValue("user.name")
			if err != nil {
				return err
			}
			email, err := gitConfigValue("user.email")
			if err != nil {
				return err
			}
			store, err := loadStore()
			if err != nil {
				return err
			}
			profileName := currentProfileName(store)
			if profileName == "" {
				profileName = "unknown"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", profileName)
			fmt.Fprintf(cmd.OutOrStdout(), "    name:%s\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "    email:%s\n", email)
			return nil
		},
	}
}

func newUseCmd() *cobra.Command {
	var useGlobal bool

	cmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "Switch git profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			scope := "local"
			if useGlobal {
				scope = "global"
			}

			store, err := loadStore()
			if err != nil {
				return err
			}
			prof, ok := store.Profiles[profileName]
			if !ok {
				return fmt.Errorf("profile not found: %s", profileName)
			}

			if scope == "global" {
				if err := runGitConfig("--global", "user.name", prof.Name); err != nil {
					return err
				}
				if err := runGitConfig("--global", "user.email", prof.Email); err != nil {
					return err
				}
			} else {
				if err := runGitConfig("user.name", prof.Name); err != nil {
					return err
				}
				if err := runGitConfig("user.email", prof.Email); err != nil {
					return err
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Switched to [%s] (%s)\n", profileName, scope)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&useGlobal, "global", "g", false, "Use global git config")

	return cmd
}

func newAddCmd() *cobra.Command {
	var name string
	var email string

	cmd := &cobra.Command{
		Use:   "add <profile>",
		Short: "Add a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			if strings.TrimSpace(name) == "" || strings.TrimSpace(email) == "" {
				return errors.New("--name and --email are required")
			}

			store, err := loadStore()
			if err != nil {
				return err
			}
			store.Profiles[profileName] = profile{Name: name, Email: email}
			if err := saveStore(store); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added profile: %s\n", profileName)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Profile user name")
	cmd.Flags().StringVar(&email, "email", "", "Profile email")

	return cmd
}

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <profile>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			store, err := loadStore()
			if err != nil {
				return err
			}
			if _, ok := store.Profiles[profileName]; !ok {
				return fmt.Errorf("profile not found: %s", profileName)
			}
			delete(store.Profiles, profileName)
			if err := saveStore(store); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed profile: %s\n", profileName)
			return nil
		},
	}
}

func loadStore() (profileStore, error) {
	path, err := storePath()
	if err != nil {
		return profileStore{}, err
	}

	store := profileStore{Profiles: map[string]profile{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			addDefaultProfile(&store)
			return store, nil
		}
		return profileStore{}, err
	}

	if err := json.Unmarshal(data, &store); err != nil {
		return profileStore{}, fmt.Errorf("parse profile store: %w", err)
	}

	if store.Profiles == nil {
		store.Profiles = map[string]profile{}
	}
	return store, nil
}

func saveStore(store profileStore) error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func addDefaultProfile(store *profileStore) {
	store.Profiles[defaultProfileName] = profile{
		Name:  "your-name",
		Email: "yourname@example.com",
	}
}

func storePath() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if strings.TrimSpace(configDir) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "gprof", "profiles.json"), nil
}

func gitConfigValue(key string) (string, error) {
	cmd := exec.Command("git", "config", key)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git config %s failed: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGitConfig(args ...string) error {
	cmd := exec.Command("git", append([]string{"config"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

func currentProfileName(store profileStore) string {
	name, err := gitConfigValue("user.name")
	if err != nil {
		return ""
	}
	email, err := gitConfigValue("user.email")
	if err != nil {
		return ""
	}
	for profileName, prof := range store.Profiles {
		if prof.Name == name && prof.Email == email {
			return profileName
		}
	}
	return ""
}
