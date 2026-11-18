// mkupdate erstellt ein Vertical3-Update-Paket.
// Aufruf: mkupdate <quellordner> <ziel.v3update> <version> <beschreibung> <datei>...
package main

import (
	"fmt"
	"os"

	"vertical3/update"
)

func main() {
	if len(os.Args) < 6 {
		fmt.Println("mkupdate <quellordner> <ziel.v3update> <version> <beschreibung> <datei>...")
		os.Exit(2)
	}
	err := update.Baue(os.Args[1], os.Args[2], update.Manifest{Version: os.Args[3], Beschreibung: os.Args[4], Dateien: os.Args[5:]})
	if err != nil {
		fmt.Println("Fehler:", err)
		os.Exit(1)
	}
	fmt.Println("erstellt:", os.Args[2])
}
