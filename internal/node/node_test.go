package node

import "testing"

func TestStripRepoPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"nosync-dot-secrets", "secrets"},
		{"nosync-work", "work"},
		{"dot-tmux.conf", "tmux.conf"},
		{"shellrc", "shellrc"},
		{"", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := StripRepoPrefix(c.in)
			if got != c.want {
				t.Errorf("StripRepoPrefix(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDeriveName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"shellrc/helpers.sh", "shellrc.helpers"},
		{"shellrc/math.sh", "shellrc.math"},
		{"tmux/shellrc/helpers.sh", "tmux.shellrc.helpers"},
		{"nosync-work/shellrc/aliases.sh", "work.shellrc.aliases"},
		{"conf/dot-tmux.conf", "conf.tmux"},
		{"nosync-dot-secrets/api.sh", "secrets.api"},
		{"conf/dot-config/tmux/tmux.conf", "conf.config.tmux.tmux"},
		{"dot-foo/shellrc/bar.sh", "foo.shellrc.bar"},
		{"nosync-dot-work/shellrc/bar.sh", "work.shellrc.bar"},
		{"shellrc/dot-aliases.sh", "shellrc.aliases"},
		{"bin/my-tool", "bin.my-tool"},
		{"aliases.sh", "aliases"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got := DeriveName(c.path)
			if got != c.want {
				t.Errorf("DeriveName(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}
