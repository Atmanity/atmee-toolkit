package cmd

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/Atmanity/toolkit/cli/internal/api"
	"github.com/Atmanity/toolkit/cli/internal/avatarapi"
	"github.com/Atmanity/toolkit/cli/internal/creds"
)

// The account's API keys, managed by the signed-in user (`atmee login`; an
// API key is refused). A secret exists only in a create's or a rotate's
// answer: with --out it goes straight into a file and is never printed.

const keysNeedLogin = "Needs a signed-in user who is the account's owner or an admin (`atmee login`); an API key cannot manage keys."

// defaultKeyVar is the variable --out writes the secret to.
const defaultKeyVar = "ATMEE_API_KEY"

func (a *app) keysCmd() *cobra.Command {
	return group("keys", "Create, edit, rotate and revoke the account's API keys (signed-in user only)",
		a.keysListCmd(), a.keysCreateCmd(), a.keysUpdateCmd(), a.keysRotateCmd(), a.keysRevokeCmd())
}

func (a *app) keysListCmd() *cobra.Command {
	var jsonOut, all bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List the account's API keys; secrets are never shown.",
		Long:  "List the account's API keys, newest first: name, id, the secret's prefix and last four characters, scopes, avatar, caps and last use. Revoked keys only with --all. " + keysNeedLogin,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, err := a.keysAPI()
			if err != nil {
				return err
			}
			keys, err := cl.APIKeys(ctx())
			if err != nil {
				return err
			}
			if !all {
				live := keys[:0]
				for _, k := range keys {
					if k.Active {
						live = append(live, k)
					}
				}
				keys = live
			}
			if jsonOut {
				return a.printJSON(keys)
			}
			if len(keys) == 0 {
				a.printf("No API keys. Create one with `atmee keys create --name NAME --out .env`.\n")
				return nil
			}
			for i, k := range keys {
				if i > 0 {
					a.printf("\n")
				}
				a.printKey(k)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "Include revoked keys")
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

func (a *app) keysCreateCmd() *cobra.Command {
	var name, out, envVar string
	var flags keyFlags
	var jsonOut bool
	c := &cobra.Command{
		Use:   "create",
		Short: "Create an API key; its secret is shown once (or written to --out).",
		Long: "Create an API key for an app's backend. Keep it narrow: only the scopes the app needs (`sessions`, starting and ending " +
			"conversations, is always included), --avatar when it serves one avatar, and a spend cap. The secret exists only in this answer: " +
			"with --out it is written to the file as " + defaultKeyVar + "=... (other lines kept, mode 0600) and never printed; without --out it " +
			"is printed once. Never put it in a browser or a repository. " + keysNeedLogin,
		Example: `  atmee keys create --name "stream app" --avatar AVATAR_ID --spend-cap 50 --out .env
  atmee keys create --name builder --scopes avatars:read,avatars:write --out .env`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(name) == "" {
				return usagef("--name is required (what the key is for, e.g. the app's name)")
			}
			req, err := flags.fields(cmd)
			if err != nil {
				return err
			}
			req["name"] = name
			path, err := checkOut(out, envVar)
			if err != nil {
				return err
			}
			cl, err := a.keysAPI()
			if err != nil {
				return err
			}
			created, err := cl.CreateAPIKey(ctx(), req)
			if err != nil {
				return err
			}
			return a.deliverSecret(created, out, path, envVar, jsonOut, "Created")
		},
	}
	c.Flags().StringVar(&name, "name", "", "What the key is for, e.g. the app's name (required)")
	flags.add(c)
	a.secretFlags(c, &out, &envVar, &jsonOut)
	return c
}

func (a *app) keysUpdateCmd() *cobra.Command {
	var name string
	var flags keyFlags
	var anyAvatar, jsonOut bool
	c := &cobra.Command{
		Use:   "update KEY_ID",
		Short: "Change a key's name, scopes, avatar and limits; its secret stays.",
		Long: "Change a key's name, scopes, avatar and limits; only the flags you pass change, and the secret stays. --scopes replaces the whole " +
			"set (`sessions` is always kept). The plan's ceilings apply again on every edit, also to limits you did not pass, so the " +
			"answer can show lower limits than before. If an edit fails partway, send it again. A revoked key answers 409 api_key_revoked. " + keysNeedLogin,
		Example: `  atmee keys update KEY_ID --name "stream app v2" --spend-cap 100
  atmee keys update KEY_ID --scopes avatars:read,generation:write --any-avatar
  atmee keys update KEY_ID --visitor-memory=false`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := api.ParseKeyID(args[0]); err != nil {
				return usagef("%v", err)
			}
			if flags.avatar != "" && anyAvatar {
				return usagef("--avatar and --any-avatar contradict each other: give one")
			}
			req, err := flags.fields(cmd)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("name") {
				if strings.TrimSpace(name) == "" {
					return usagef("--name cannot be empty")
				}
				req["name"] = name
			}
			if anyAvatar {
				req["avatarId"] = nil
			}
			if len(req) == 0 {
				return usagef("nothing to update: pass at least one flag")
			}
			cl, err := a.keysAPI()
			if err != nil {
				return err
			}
			k, err := cl.UpdateAPIKey(ctx(), args[0], req)
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(k)
			}
			a.printf("Updated. The key as saved:\n")
			a.printKey(*k)
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", "", "A new name")
	flags.add(c)
	c.Flags().BoolVar(&anyAvatar, "any-avatar", false, "Widen the key to every avatar of the account")
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

// keyFlags are the settings keys create and keys update share.
type keyFlags struct {
	scopes         []string
	avatar, window string
	maxSession     time.Duration
	maxConcurrent  int
	spendCap       float64
	visitorMemory  bool
}

func (k *keyFlags) add(c *cobra.Command) {
	f := c.Flags()
	f.StringSliceVar(&k.scopes, "scopes", nil, "Scopes besides sessions: avatars:read, avatars:write, billing:write, generation:write (comma-separated)")
	f.StringVar(&k.avatar, "avatar", "", "Restrict the key to this one avatar")
	f.DurationVar(&k.maxSession, "max-session", 0, "Longest session the key may start, e.g. 15m (default: the plan's)")
	f.IntVar(&k.maxConcurrent, "max-concurrent", 0, "Sessions the key may run at once (default: the plan's)")
	f.Float64Var(&k.spendCap, "spend-cap", 0, "Credits the key may spend per window (default 500)")
	f.StringVar(&k.window, "spend-cap-window", "", "daily, monthly (default) or total")
	f.BoolVar(&k.visitorMemory, "visitor-memory", false, "Let the key's sessions remember a visitor across sessions (visitorId)")
}

// fields checks the flags given and returns them under the API's names.
func (k *keyFlags) fields(cmd *cobra.Command) (map[string]any, error) {
	f := cmd.Flags()
	out := map[string]any{}
	if f.Changed("scopes") {
		list := make([]string, 0, len(k.scopes))
		for _, s := range k.scopes {
			s = strings.TrimSpace(s)
			if !avatarapi.ApiKeyCreateRequestScopes(s).Valid() {
				return nil, usagef("--scopes: unknown scope %q (sessions, avatars:read, avatars:write, billing:write, generation:write)", s)
			}
			list = append(list, s)
		}
		out["scopes"] = list
	}
	if k.avatar != "" {
		id, err := uuid.Parse(k.avatar)
		if err != nil {
			return nil, usagef("--avatar: %q is not an avatar id (find it with `atmee avatars list`)", k.avatar)
		}
		out["avatarId"] = id
	}
	if f.Changed("max-session") {
		secs := int(k.maxSession.Seconds())
		if secs < 30 || secs > 14400 {
			return nil, usagef("--max-session takes 30s to 4h")
		}
		out["maxSessionSeconds"] = secs
	}
	if f.Changed("max-concurrent") {
		if k.maxConcurrent < 1 || k.maxConcurrent > 100 {
			return nil, usagef("--max-concurrent takes 1 to 100")
		}
		out["maxConcurrentSessions"] = k.maxConcurrent
	}
	if f.Changed("spend-cap") {
		microcredits := math.Round(k.spendCap * 100) // 100 microcredits = 1 credit
		if math.IsNaN(microcredits) || microcredits < 1 || microcredits > math.MaxInt32 {
			return nil, usagef("--spend-cap takes a positive number of credits")
		}
		out["spendCapMicrocredits"] = int(microcredits)
	}
	if k.window != "" {
		if !avatarapi.ApiKeyCreateRequestSpendCapWindow(k.window).Valid() {
			return nil, usagef("--spend-cap-window takes daily, monthly or total")
		}
		out["spendCapWindow"] = k.window
	}
	if f.Changed("visitor-memory") {
		out["visitorMemory"] = k.visitorMemory
	}
	return out, nil
}

func (a *app) keysRotateCmd() *cobra.Command {
	var out, envVar string
	var jsonOut bool
	c := &cobra.Command{
		Use:   "rotate KEY_ID",
		Short: "Issue a new secret for a key; the old one stops working within about 30 seconds.",
		Long: "Issue a new secret for a key: it keeps its id, name, scopes, avatar and caps, and the old secret stops working within about " +
			"30 seconds. With --out the new secret is written to the file as " + defaultKeyVar + "=... and never printed. " + keysNeedLogin,
		Example: `  atmee keys rotate KEY_ID --out .env`,
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := checkOut(out, envVar)
			if err != nil {
				return err
			}
			cl, err := a.keysAPI()
			if err != nil {
				return err
			}
			rotated, err := cl.RotateAPIKey(ctx(), args[0])
			if err != nil {
				return err
			}
			return a.deliverSecret(rotated, out, path, envVar, jsonOut, "Rotated")
		},
	}
	a.secretFlags(c, &out, &envVar, &jsonOut)
	return c
}

func (a *app) keysRevokeCmd() *cobra.Command {
	var confirm, jsonOut bool
	c := &cobra.Command{
		Use:   "revoke KEY_ID",
		Short: "Revoke a key for good (asks first; --yes skips the question).",
		Long:  "Revoke a key for good: it stops working within about 30 seconds; sessions it started keep running. " + keysNeedLogin,
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := api.ParseKeyID(args[0]); err != nil {
				return usagef("%v", err)
			}
			if !confirm {
				if err := a.confirm(fmt.Sprintf("Revoke API key %s? Apps using it stop working. [y/N]", args[0])); err != nil {
					return err
				}
			}
			cl, err := a.keysAPI()
			if err != nil {
				return err
			}
			k, err := cl.RevokeAPIKey(ctx(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return a.printJSON(k)
			}
			a.printf("Revoked %s (%s).\n", k.Name, k.Id)
			return nil
		},
	}
	c.Flags().BoolVar(&confirm, "yes", false, "Revoke without asking")
	c.Flags().BoolVar(&jsonOut, "json", false, jsonHelp)
	return c
}

// keysAPI is the client for the key commands: a signed-in user only.
func (a *app) keysAPI() (*api.Client, error) { return a.loginAPI("managing API keys") }

// loginAPI is the client for what only a signed-in user may do.
func (a *app) loginAPI(what string) (*api.Client, error) {
	if a.store.APIKey() != "" {
		return nil, usagef("%s needs a signed-in user, but an API key is in use (ATMEE_API_KEY, or one saved by `atmee login --api-key`): unset ATMEE_API_KEY and run `atmee login`", what)
	}
	return a.api()
}

func (a *app) secretFlags(c *cobra.Command, out, envVar *string, jsonOut *bool) {
	c.Flags().StringVar(out, "out", "", "Write the secret to this env file (e.g. .env) instead of printing it")
	c.Flags().StringVar(envVar, "env-var", defaultKeyVar, "The variable --out writes")
	c.Flags().BoolVar(jsonOut, "json", false, "Print the API's JSON (the secret is left out with --out)")
}

const secretWarning = "This secret is shown only now. Store it in your backend's secret store or .env (never in a browser or a repository); next time use --out FILE to write it without printing.\n"

// deliverSecret puts a new secret where it belongs: into the --out file
// (path, the file it resolves to; never printed), or printed once with a
// warning.
func (a *app) deliverSecret(s *api.KeySecret, out, path, envVar string, jsonOut bool, verb string) error {
	where := "."
	if out != "" {
		if err := writeEnvVar(path, envVar, s.ApiKey); err != nil {
			return fmt.Errorf("%s key %s, but writing its secret to %s failed: %w (rotate it with `atmee keys rotate %s --out FILE` to get a new secret)",
				strings.ToLower(verb), s.Key.Id, out, err, s.Key.Id)
		}
		where = fmt.Sprintf("; its secret is in %s as %s.", out, envVar)
	} else {
		a.note(secretWarning)
	}
	if jsonOut {
		if out != "" {
			return a.printJSON(s.Key)
		}
		if err := a.printJSON(s); err != nil {
			return fmt.Errorf("%s key %s, but printing its secret failed: %w (rotate it with `atmee keys rotate %s --out FILE` to get a new secret)",
				strings.ToLower(verb), s.Key.Id, err, s.Key.Id)
		}
		return nil
	}
	a.printf("%s key %q (%s)%s\n", verb, s.Key.Name, s.Key.Id, where)
	a.printKey(s.Key)
	if out == "" {
		// The only copy of the secret: a failed write is a failed command.
		if _, err := fmt.Fprintf(a.out, "\n%s=%s\n", envVar, s.ApiKey); err != nil {
			return fmt.Errorf("%s key %s, but printing its secret failed: %w (rotate it with `atmee keys rotate %s --out FILE` to get a new secret)",
				strings.ToLower(verb), s.Key.Id, err, s.Key.Id)
		}
	}
	return nil
}

func (a *app) printKey(k api.Key) {
	state := ""
	if !k.Active {
		state = "  REVOKED"
		if k.RevokedAt != nil {
			state += " " + day(*k.RevokedAt)
		}
	}
	a.printf("%s  %s%s\n", k.Name, k.Id, state)
	a.printf("  Secret      %s…%s\n", k.KeyPrefix, k.LastFour)
	scopes := make([]string, 0, len(k.Scopes))
	for _, s := range k.Scopes {
		scopes = append(scopes, string(s))
	}
	a.printf("  Scopes      %s\n", strings.Join(scopes, ", "))
	avatar := "any avatar of the account"
	if k.AvatarId != nil {
		avatar = "only " + k.AvatarId.String()
	}
	a.printf("  Avatars     %s\n", avatar)
	window := map[string]string{"daily": "a day", "monthly": "a month", "total": "in total"}[string(k.SpendCapWindow)]
	length := "the plan's length"
	if k.MaxSessionSeconds != nil {
		length = (time.Duration(*k.MaxSessionSeconds) * time.Second).String()
	}
	a.printf("  Limits      %s credits %s · sessions up to %s · %d at once\n", credits(int64(k.SpendCapMicrocredits)), window,
		length, k.MaxConcurrentSessions)
	used := "never used"
	if k.LastUsedAt != nil {
		used = "last used " + day(*k.LastUsedAt)
	}
	a.printf("  Created     %s · %s\n", day(k.CreatedAt), used)
}

// checkOut refuses an --env-var or --out the secret could not be written
// as or to, before anything is sent: a failure after a create or a rotate
// costs a key. It returns the file --out resolves to.
func checkOut(out, envVar string) (string, error) {
	if err := checkEnvVar(envVar); err != nil {
		return "", err
	}
	if out == "" {
		return "", nil
	}
	path, err := envFilePath(out)
	if err != nil {
		return "", usagef("--out %s: %v", out, err)
	}
	if fi, err := os.Stat(path); err == nil {
		if fi.IsDir() {
			return "", usagef("--out %s is a directory; name a file such as %s", out, filepath.Join(out, ".env"))
		}
		if !fi.Mode().IsRegular() {
			return "", usagef("--out %s is not a regular file", out)
		}
		// Its other lines are kept, so it must be readable now, not after
		// the secret exists.
		if _, err := os.ReadFile(path); err != nil {
			return "", usagef("--out %s cannot be read: %v", out, err)
		}
	}
	probe, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", usagef("--out %s cannot be written: %v", out, err)
	}
	probe.Close()
	_ = os.Remove(probe.Name())
	return path, nil
}

// envFilePath is the file --out names, followed through symlinks (even to
// a target that does not exist yet): the target gets the secret and the
// link stays.
func envFilePath(out string) (string, error) {
	path := out
	for range 40 {
		fi, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
	return "", errors.New("too many levels of symbolic links")
}

// checkEnvVar refuses a variable name a shell or dotenv file would not read.
func checkEnvVar(name string) error {
	if name == "" {
		return usagef("--env-var is empty")
	}
	for i, r := range name {
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9')) {
			return usagef("--env-var %q is not a variable name", name)
		}
	}
	return nil
}

// writeEnvVar sets NAME=value in an env file: the line that sets NAME is
// replaced (or one is added), every other line is kept, and the file is
// written whole through a temporary file, readable by its owner only.
func writeEnvVar(path, name, value string) error {
	var lines []string
	if old, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(string(old), "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	line := name + "=" + value
	replaced := false
	for i, l := range lines {
		t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "export "))
		if k, _, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == name {
			lines[i], replaced = line, true
		}
	}
	if !replaced {
		lines = append(lines, line)
	}
	return creds.WriteSecretFile(path, []byte(strings.Join(lines, "\n")+"\n"))
}
