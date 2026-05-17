package adopter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInfer_Executable(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "my-script")
	if err := os.WriteFile(f, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	got := Infer(f, info, DefaultConventions())
	if got.Unknown {
		t.Fatal("expected non-unknown inference")
	}
	if got.DestRel != "bin/my-script" {
		t.Errorf("DestRel = %q, want %q", got.DestRel, "bin/my-script")
	}
}

func TestInfer_ShellExt(t *testing.T) {
	exts := []string{".sh", ".bash", ".zsh", ".fish"}
	for _, ext := range exts {
		t.Run(ext, func(t *testing.T) {
			tmp := t.TempDir()
			f := filepath.Join(tmp, "aliases"+ext)
			if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
				t.Fatal(err)
			}
			info, _ := os.Stat(f)
			got := Infer(f, info, DefaultConventions())
			want := "shellrc/aliases" + ext
			if got.DestRel != want {
				t.Errorf("DestRel = %q, want %q", got.DestRel, want)
			}
		})
	}
}

func TestInfer_HiddenFile(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, ".bashrc")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(f)
	got := Infer(f, info, DefaultConventions())
	if got.DestRel != "conf/dot-bashrc" {
		t.Errorf("DestRel = %q, want %q", got.DestRel, "conf/dot-bashrc")
	}
}

func TestInfer_ConfigExt(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"starship.toml", "conf/starship.toml"},
		{"app.yaml", "conf/app.yaml"},
		{"settings.json", "conf/settings.json"},
		{"app.conf", "conf/app.conf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			f := filepath.Join(tmp, tc.name)
			if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
				t.Fatal(err)
			}
			info, _ := os.Stat(f)
			got := Infer(f, info, DefaultConventions())
			if got.DestRel != tc.want {
				t.Errorf("DestRel = %q, want %q", got.DestRel, tc.want)
			}
		})
	}
}

func TestInfer_Unknown(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "README")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(f)
	got := Infer(f, info, DefaultConventions())
	if !got.Unknown {
		t.Errorf("expected Unknown=true for plain file with no extension, got DestRel=%q", got.DestRel)
	}
}

func TestInfer_CustomConventions(t *testing.T) {
	conv := ConventionNames{Shellrc: "scripts", Bin: "executables", Conf: "dotfiles"}
	tmp := t.TempDir()
	f := filepath.Join(tmp, ".gitconfig")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(f)
	got := Infer(f, info, conv)
	if got.DestRel != "dotfiles/dot-gitconfig" {
		t.Errorf("DestRel = %q, want %q", got.DestRel, "dotfiles/dot-gitconfig")
	}
}
