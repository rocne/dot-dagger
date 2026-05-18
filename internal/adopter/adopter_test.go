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

func TestAdopt_Conf(t *testing.T) {
	dotfiles := t.TempDir()
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, ".bashrc")
	if err := os.WriteFile(src, []byte("# bashrc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := AdoptOptions{
		DotfilesRoot: dotfiles,
		Conventions:  DefaultConventions(),
		LinkRoot:     srcDir,
	}

	res, err := Adopt(src, "conf/dot-bashrc", opts)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	// Original regular file replaced by symlink.
	if fi, statErr := os.Lstat(src); statErr != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("original file should have been replaced by a symlink")
	}

	// File present in dotfiles.
	destAbs := filepath.Join(dotfiles, "conf/dot-bashrc")
	content, err := os.ReadFile(destAbs)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(content) != "# bashrc\n" {
		t.Errorf("dest content = %q, want %q", content, "# bashrc\n")
	}

	// Symlink at original src path → destAbs.
	target, err := os.Readlink(src)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != destAbs {
		t.Errorf("symlink target = %q, want %q", target, destAbs)
	}

	// ActResult has one link.
	if len(res.Links) != 1 {
		t.Fatalf("res.Links len = %d, want 1", len(res.Links))
	}
	if res.Links[0].Src != destAbs || res.Links[0].Dest != src {
		t.Errorf("link = %+v, want Src=%q Dest=%q", res.Links[0], destAbs, src)
	}
}

func TestAdopt_Bin(t *testing.T) {
	dotfiles := t.TempDir()
	binDir := t.TempDir()
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "my-script")
	if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	opts := AdoptOptions{
		DotfilesRoot: dotfiles,
		Conventions:  DefaultConventions(),
		BinDir:       binDir,
	}

	res, err := Adopt(src, "bin/my-script", opts)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	// Original removed.
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Error("original file should have been removed")
	}

	destAbs := filepath.Join(dotfiles, "bin/my-script")
	symlinkPath := filepath.Join(binDir, "my-script")

	// Symlink at binDir/my-script → destAbs.
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != destAbs {
		t.Errorf("symlink target = %q, want %q", target, destAbs)
	}

	if len(res.Links) != 1 {
		t.Fatalf("res.Links len = %d, want 1", len(res.Links))
	}
}

func TestAdopt_Shellrc(t *testing.T) {
	dotfiles := t.TempDir()
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "aliases.sh")
	if err := os.WriteFile(src, []byte("alias ll='ls -la'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := AdoptOptions{
		DotfilesRoot: dotfiles,
		Conventions:  DefaultConventions(),
	}

	res, err := Adopt(src, "shellrc/aliases.sh", opts)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	// Original removed.
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Error("original file should have been removed")
	}

	// No symlinks — shellrc files are sourced, not linked.
	if len(res.Links) != 0 {
		t.Errorf("res.Links = %v, want empty", res.Links)
	}
	// Node appears in Sourced.
	if len(res.Sourced) != 1 {
		t.Errorf("res.Sourced len = %d, want 1", len(res.Sourced))
	}

	// File present in dotfiles.
	destAbs := filepath.Join(dotfiles, "shellrc/aliases.sh")
	if _, err := os.Stat(destAbs); err != nil {
		t.Errorf("dest missing: %v", err)
	}
}

func TestAdopt_DestExists(t *testing.T) {
	dotfiles := t.TempDir()
	destAbs := filepath.Join(dotfiles, "conf/dot-bashrc")
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destAbs, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(t.TempDir(), ".bashrc")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := AdoptOptions{DotfilesRoot: dotfiles, Conventions: DefaultConventions()}
	_, err := Adopt(src, "conf/dot-bashrc", opts)
	if err == nil {
		t.Fatal("expected error when dest exists, got nil")
	}
}

func TestAdopt_DryRun(t *testing.T) {
	dotfiles := t.TempDir()
	src := filepath.Join(t.TempDir(), ".bashrc")
	if err := os.WriteFile(src, []byte("# rc"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := AdoptOptions{
		DotfilesRoot: dotfiles,
		Conventions:  DefaultConventions(),
		DryRun:       true,
	}

	_, err := Adopt(src, "conf/dot-bashrc", opts)
	if err != nil {
		t.Fatalf("Adopt dry-run: %v", err)
	}

	// Source file still present.
	if _, statErr := os.Stat(src); statErr != nil {
		t.Error("original should still exist in dry-run")
	}

	// Dest NOT created.
	destAbs := filepath.Join(dotfiles, "conf/dot-bashrc")
	if _, statErr := os.Stat(destAbs); !os.IsNotExist(statErr) {
		t.Error("dest should not be created in dry-run")
	}
}
