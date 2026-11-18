package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FindeExiftool sucht exiftool im Programmordner oder im Suchpfad.
func FindeExiftool(appDir string) string {
	for _, n := range []string{"exiftool.exe", "exiftool(-k).exe", "exiftool"} {
		p := filepath.Join(appDir, n)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("exiftool"); err == nil {
		return p
	}
	return ""
}

// Exiftool ergänzt die Daten um alles, was exiftool liest (falls vorhanden).
func Exiftool(exe, pfad string, d Daten) error {
	if exe == "" {
		return nil
	}
	ctx, abbruch := context.WithTimeout(context.Background(), 60*time.Second)
	defer abbruch()
	cmd := exec.CommandContext(ctx, exe, "-j", "-n", "-G1", "-charset", "filename=utf8", pfad)
	versteckeFenster(cmd)
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	var liste []map[string]any
	if err := json.Unmarshal(out, &liste); err != nil || len(liste) == 0 {
		return err
	}
	for k, v := range liste[0] {
		var s string
		switch x := v.(type) {
		case string:
			s = x
		case float64:
			s = fmt.Sprint(x)
		case bool:
			s = fmt.Sprint(x)
		default:
			continue
		}
		d[k] = s
		kurz := k
		if i := strings.LastIndex(k, ":"); i >= 0 {
			kurz = k[i+1:]
		}
		if _, da := d[kurz]; !da {
			d[kurz] = s
		}
	}
	return nil
}
