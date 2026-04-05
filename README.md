# gprof

A small CLI to manage Git user profiles and switch `user.name` / `user.email` quickly.

### install

```bash
go install github.com/hxzll/gprof@latest
```

## Usage

```bash
gprof list
gprof list -d

gprof current

gprof use personal
gprof use work -g

gprof add profile1 --name user1 --email user1@example.com

gprof remove profile1
```

## Profiles

Profiles are stored in the XDG config directory:

- `$XDG_CONFIG_HOME/gprof/profiles.json`
- fallback: `~/.config/gprof/profiles.json`

`personal` exists on first run, but can be removed and will not be recreated.

## Notes

- `gprof add` creates or overwrites a profile with the same name.
- `gprof use` defaults to local scope. Use `-g` for global.
