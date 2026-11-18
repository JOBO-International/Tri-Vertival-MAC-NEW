package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"

	"vertical3/update"
)

func main() {
	d, _ := os.MkdirTemp("", "upd")
	app := filepath.Join(d, "app")
	src := filepath.Join(d, "src")
	os.MkdirAll(filepath.Join(app, "kameraprofile"), 0o755)
	os.MkdirAll(filepath.Join(src, "kameraprofile"), 0o755)
	os.WriteFile(filepath.Join(app, "Vertical3.exe"), []byte("MZ alte Version"), 0o755)
	os.WriteFile(filepath.Join(app, "version.txt"), []byte("2.0"), 0o644)
	os.WriteFile(filepath.Join(src, "Vertical3.exe"), []byte("MZ neue Version"), 0o755)
	os.WriteFile(filepath.Join(src, "kameraprofile", "neu.json"), []byte(`{"name":"x"}`), 0o644)
	os.WriteFile(filepath.Join(src, "falsch.exe"), []byte("kein programm"), 0o755)
	gut := filepath.Join(d, "gut.v3update")
	fmt.Println("bauen:", update.Baue(src, gut, update.Manifest{Version: "2.0.1", Beschreibung: "Test", Dateien: []string{"Vertical3.exe", "kameraprofile/neu.json"}}))
	alt := filepath.Join(d, "alt.v3update")
	update.Baue(src, alt, update.Manifest{Version: "1.9", Dateien: []string{"Vertical3.exe"}})
	// böse Pakete
	boese := filepath.Join(d, "boese.v3update")
	f, _ := os.Create(boese)
	z := zip.NewWriter(f)
	w, _ := z.Create("update.json")
	w.Write([]byte(`{"programm":"Vertical3","version":"9.0","dateien":["../evil.exe"]}`))
	z.Close()
	f.Close()
	keinexe := filepath.Join(d, "keinexe.v3update")
	f, _ = os.Create(keinexe)
	z = zip.NewWriter(f)
	w, _ = z.Create("update.json")
	w.Write([]byte(`{"programm":"Vertical3","version":"9.0","dateien":["Vertical3.exe"]}`))
	w, _ = z.Create("Vertical3.exe")
	w.Write([]byte("kein programm"))
	z.Close()
	f.Close()
	os.WriteFile(filepath.Join(d, "muell.v3update"), []byte("x"), 0o644)
	ver := func() string { return update.InstallierteVersion(app, "?") }
	for _, p := range []string{"alt", "boese", "keinexe", "muell", "gut"} {
		_, err := update.Installiere(filepath.Join(d, p+".v3update"), app, ver())
		fmt.Printf("%-8s -> %v | Version %s\n", p, err, ver())
	}
	exe, _ := os.ReadFile(filepath.Join(app, "Vertical3.exe"))
	_, e := os.Stat(filepath.Join(app, "kameraprofile", "neu.json"))
	_, a := os.Stat(filepath.Join(app, "Vertical3.exe.alt"))
	fmt.Printf("exe: %q, neues Profil da: %v, alte exe umbenannt: %v\n", exe, e == nil, a == nil)
	update.Aufraeumen(app)
	v, err := update.Rueckgaengig(app)
	exe, _ = os.ReadFile(filepath.Join(app, "Vertical3.exe"))
	_, e = os.Stat(filepath.Join(app, "kameraprofile", "neu.json"))
	fmt.Printf("rückgängig -> %s %v | exe: %q, Profil noch da: %v, Version %s\n", v, err, exe, e == nil, ver())
	os.RemoveAll(d)
}
