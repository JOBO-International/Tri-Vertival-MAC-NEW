// Erzeugt rsrc_windows_amd64.syso mit Manifest, Symbol und Versionsinfo.
package main

import (
	"image"
	_ "image/png"
	"os"
	"strings"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func main() {
	out, icoPath, ver := os.Args[1], os.Args[2], os.Args[3]
	rs := &winres.ResourceSet{}
	rs.SetManifest(winres.AppManifest{
		Identity:            winres.AssemblyIdentity{Name: "Vertical3"},
		Description:         "Vertical3 Perspektivkorrektur",
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
		ExecutionLevel:      winres.AsInvoker,
	})
	f, err := os.Open(icoPath)
	if err != nil {
		panic(err)
	}
	var icon *winres.Icon
	if strings.HasSuffix(strings.ToLower(icoPath), ".ico") {
		// fertiges Mehrgrößen-Symbol (kleine Größen mit kräftigeren Strichen)
		icon, err = winres.LoadICO(f)
	} else {
		var img image.Image
		img, _, err = image.Decode(f)
		if err == nil {
			icon, err = winres.NewIconFromResizedImage(img, nil)
		}
	}
	if err != nil {
		panic(err)
	}
	rs.SetIcon(winres.Name("APPICON"), icon)
	vi := version.Info{}
	vi.SetFileVersion(ver)
	vi.SetProductVersion(ver)
	// Hauptprogramm heißt Tri-Vertical, Zusatzprogramme behalten ihren Namen
	produkt, beschreibung := "Tri-Vertical", "Tri-Vertical – Perspektivkorrektur"
	if len(os.Args) > 4 {
		produkt, beschreibung = "Tri-Vertical", os.Args[4]
	}
	vi.Set(version.LangDefault, version.ProductName, produkt)
	vi.Set(version.LangDefault, version.FileDescription, beschreibung)
	vi.Set(version.LangDefault, version.CompanyName, "privat")
	rs.SetVersionInfo(vi)
	o, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer o.Close()
	if err := rs.WriteObject(o, winres.ArchAMD64); err != nil {
		panic(err)
	}
}
