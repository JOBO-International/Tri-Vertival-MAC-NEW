package filter

// Look: vordefinierter oder eigener Filter.
type Look struct {
	Name      string      `json:"name"`
	Gruppe    string      `json:"gruppe"`
	Werte     Einstellung `json:"werte"`
	Eigener   bool        `json:"-"`
	Dateipfad string      `json:"-"`
}

func mit(f func(e *Einstellung)) Einstellung {
	e := Neutral()
	f(&e)
	return e
}

// Vordefiniert: die mitgelieferten Looks.
var Vordefiniert = []Look{
	{Name: "Original", Gruppe: "Farbe", Werte: Neutral()},
	{Name: "Lebendig", Gruppe: "Farbe", Werte: mit(func(e *Einstellung) {
		e.Saettigung, e.Dynamik, e.Kontrast, e.Klarheit = 0.2, 0.4, 0.2, 0.15
	})},
	{Name: "Weich & hell", Gruppe: "Farbe", Werte: mit(func(e *Einstellung) {
		e.Belichtung, e.Kontrast, e.Lichter, e.Tiefen, e.Matt, e.Saettigung = 0.2, -0.3, -0.3, 0.35, 0.25, -0.1
	})},
	{Name: "Herbst", Gruppe: "Farbe", Werte: mit(func(e *Einstellung) {
		e.Temperatur, e.Saettigung, e.Dynamik, e.Kontrast = 0.35, 0.15, 0.25, 0.1
		e.Kanal[0].Licht, e.Kanal[1].Licht = 0.15, -0.05
	})},
	{Name: "Nordisch", Gruppe: "Farbe", Werte: mit(func(e *Einstellung) {
		e.Temperatur, e.Saettigung, e.Belichtung, e.Kontrast, e.Matt = -0.25, -0.4, 0.1, -0.05, 0.2
	})},
	{Name: "Dramatisch", Gruppe: "Farbe", Werte: mit(func(e *Einstellung) {
		e.Kontrast, e.Klarheit, e.Saettigung, e.Vignette, e.Belichtung, e.Lichter = 0.4, 0.45, -0.2, 0.5, -0.1, -0.3
	})},
	{Name: "HDR-Look", Gruppe: "Farbe", Werte: mit(func(e *Einstellung) {
		e.Klarheit, e.Tiefen, e.Lichter, e.Dynamik, e.Kontrast = 0.8, 0.6, -0.6, 0.35, 0.1
	})},
	{Name: "Natürlich", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		e.Kontrast, e.Saettigung, e.Tiefen, e.Lichter = -0.1, -0.05, 0.1, -0.1
	})},
	{Name: "Analogkino", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		// analoger Kinofilm: weiche Lichter, leicht verblasst, warm, zurückhaltende Farben
		e.Matt, e.Kontrast, e.Saettigung, e.Temperatur, e.Lichter, e.Koernung = 0.25, 0.1, -0.2, 0.1, -0.25, 0.15
		e.Kanal[0].Licht, e.Kanal[2].Tiefe = 0.1, 0.12
	})},
	{Name: "Zeitgenössisch", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		// klar und modern: etwas kühler, knackig, Tiefen leicht petrol
		e.Temperatur, e.Kontrast, e.Klarheit, e.Saettigung = -0.1, 0.2, 0.15, -0.05
		e.TonTiefen, e.TonLichter, e.TonStaerke = [3]float64{0.4, 0.5, 0.56}, [3]float64{0.52, 0.5, 0.48}, 0.4
	})},
	{Name: "Intensiv", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		// hoher Kontrast, kräftige Farben, Hauch Magenta
		e.Kontrast, e.Saettigung, e.Toenung, e.Dynamik, e.Vignette = 0.45, 0.3, 0.12, 0.1, 0.1
	})},
	{Name: "Diafilm", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		// gedämpfte Farben wie ein älterer Diafilm
		e.Saettigung, e.Kontrast, e.Temperatur, e.Lichter = -0.3, 0.25, -0.05, -0.1
		e.Kanal[2].Tiefe, e.Kanal[1].Licht = 0.12, 0.05
	})},
	{Name: "Bleichbad", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		// kühle Blautöne, stark reduzierte Sättigung, harter Kontrast
		e.Saettigung, e.Kontrast, e.Temperatur, e.Klarheit, e.Lichter = -0.55, 0.45, -0.25, 0.25, -0.1
	})},
	{Name: "Goldton", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		// warmes Licht wie zur goldenen Stunde
		e.Temperatur, e.Saettigung, e.Kontrast = 0.4, 0.05, 0.1
		e.TonLichter, e.TonStaerke = [3]float64{0.7, 0.6, 0.4}, 0.4
	})},
	{Name: "Petrol", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		// Blaugrün-Töne, filmisch
		e.TonTiefen, e.TonLichter, e.TonStaerke, e.Saettigung, e.Kontrast = [3]float64{0.3, 0.55, 0.58}, [3]float64{0.45, 0.58, 0.58}, 0.7, -0.15, 0.15
	})},
	{Name: "Blauton", Gruppe: "Kamera-Looks", Werte: mit(func(e *Einstellung) {
		// stilvoller Blauton bei stark zurückgenommenen Farben
		e.TonTiefen, e.TonLichter, e.TonStaerke, e.Saettigung, e.Kontrast = [3]float64{0.35, 0.45, 0.65}, [3]float64{0.45, 0.5, 0.62}, 0.7, -0.45, 0.1
	})},
	// Analoge Farbnegativfilme (angelehnt an typische Filmcharakteristiken)
	{Name: "Porträtfilm 160", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		// weiche Kontraste, natürliche Hauttöne, pastellige Farben
		e.Kontrast, e.Saettigung, e.Temperatur, e.Lichter, e.Tiefen, e.Matt, e.Koernung = -0.15, -0.12, 0.08, -0.2, 0.1, 0.08, 0.08
		e.Kanal[0].Licht, e.Kanal[2].Tiefe = 0.05, 0.04
	})},
	{Name: "Porträtfilm 400", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		// etwas wärmer und körniger, sanfte Lichter
		e.Kontrast, e.Saettigung, e.Temperatur, e.Lichter, e.Tiefen, e.Matt, e.Koernung = -0.1, -0.05, 0.12, -0.25, 0.12, 0.06, 0.16
		e.Kanal[0].Licht, e.Kanal[1].Licht = 0.06, 0.02
	})},
	{Name: "Alltagsfilm 200", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		// warm, gelbliche Lichter, kräftige Farben – der typische Urlaubsfilm
		e.Temperatur, e.Saettigung, e.Kontrast, e.Koernung = 0.25, 0.15, 0.1, 0.15
		e.Kanal[0].Licht, e.Kanal[1].Licht = 0.08, 0.04
		e.TonLichter, e.TonStaerke = [3]float64{0.6, 0.56, 0.42}, 0.25
	})},
	{Name: "Grünstichfilm 400", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		// leicht grünliche Mitten, kühle Schatten, deutliches Korn
		e.Toenung, e.Temperatur, e.Saettigung, e.Kontrast, e.Koernung = -0.12, -0.05, 0.1, 0.15, 0.18
		e.Kanal[1].Tiefe, e.Kanal[2].Licht = 0.08, -0.05
	})},
	{Name: "Brillantnegativ 100", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		// feinkörnig, satte Farben, klare Kontraste
		e.Saettigung, e.Dynamik, e.Kontrast, e.Temperatur, e.Klarheit, e.Koernung = 0.35, 0.2, 0.25, 0.05, 0.1, 0.04
	})},
	{Name: "Kunstlichtfilm 500T", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		// Kinofilm für Kunstlicht bei Tageslicht: kühl, petrolfarbene Schatten, warme Lichter
		e.Temperatur, e.Toenung, e.Kontrast, e.Lichter, e.Saettigung, e.Koernung = -0.3, 0.05, 0.1, -0.15, -0.05, 0.2
		e.TonTiefen, e.TonLichter, e.TonStaerke = [3]float64{0.38, 0.52, 0.6}, [3]float64{0.62, 0.52, 0.45}, 0.5
	})},
	{Name: "Abgelaufener Film", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		// verblasst, Farbstich ins Magenta, flaue Schwärzen, grobes Korn
		e.Matt, e.Toenung, e.Saettigung, e.Kontrast, e.Koernung, e.Vignette, e.Temperatur = 0.35, 0.12, -0.25, -0.1, 0.35, 0.3, 0.1
		e.Kanal[2].Tiefe, e.Kanal[0].Licht = 0.2, 0.1
	})},
	{Name: "Film warm", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		e.Temperatur, e.Kontrast, e.Matt, e.Saettigung, e.Koernung = 0.25, 0.1, 0.25, -0.1, 0.3
		e.Kanal[0].Licht, e.Kanal[2].Tiefe = 0.2, 0.15
	})},
	{Name: "Film kühl", Gruppe: "Farbnegativ", Werte: mit(func(e *Einstellung) {
		e.Temperatur, e.Toenung, e.Matt, e.Saettigung, e.Koernung = -0.2, -0.08, 0.2, -0.15, 0.25
		e.Kanal[2].Tiefe, e.Kanal[1].Licht = 0.25, 0.1
	})},
	// Diafilme (Umkehrfilme)
	{Name: "Landschaftsdia 50", Gruppe: "Diafilm", Werte: mit(func(e *Einstellung) {
		// sehr satte Farben, tiefe Blau- und Grüntöne, harter Kontrast
		e.Saettigung, e.Dynamik, e.Kontrast, e.Toenung, e.Tiefen, e.Klarheit, e.Koernung = 0.5, 0.2, 0.35, 0.04, -0.1, 0.1, 0.02
	})},
	{Name: "Neutraldia 100", Gruppe: "Diafilm", Werte: mit(func(e *Einstellung) {
		// ausgewogene Farben, feines Korn
		e.Saettigung, e.Kontrast, e.Koernung = 0.12, 0.2, 0.03
	})},
	{Name: "Kühldia 100", Gruppe: "Diafilm", Werte: mit(func(e *Einstellung) {
		// klar und kühl, saubere Weißtöne
		e.Temperatur, e.Saettigung, e.Kontrast, e.Klarheit = -0.12, 0.15, 0.2, 0.05
	})},
	{Name: "Klassikdia 64", Gruppe: "Diafilm", Werte: mit(func(e *Einstellung) {
		// kräftige Rottöne, warme Lichter, tiefe Schatten – der Klassiker der Reportage
		e.Kontrast, e.Saettigung, e.Temperatur, e.Tiefen, e.Lichter, e.Koernung = 0.3, 0.15, 0.08, -0.15, -0.1, 0.06
		e.Kanal[0].Licht = 0.06
		e.TonTiefen, e.TonStaerke = [3]float64{0.48, 0.48, 0.56}, 0.3
	})},
	{Name: "Cross", Gruppe: "Diafilm", Werte: mit(func(e *Einstellung) {
		e.Kontrast, e.Saettigung = 0.3, 0.2
		e.Kanal[0].Licht, e.Kanal[1].Tiefe, e.Kanal[2].Tiefe, e.Kanal[2].Licht = 0.3, -0.1, 0.35, -0.35
	})},
	// Sofortbild
	{Name: "Sofortbild", Gruppe: "Sofortbild", Werte: mit(func(e *Einstellung) {
		e.Matt, e.Temperatur, e.Toenung, e.Saettigung, e.Kontrast, e.Vignette = 0.35, 0.15, 0.06, -0.1, -0.05, 0.25
		e.Kanal[2].Tiefe = 0.25
	})},
	{Name: "Sofortbild warm", Gruppe: "Sofortbild", Werte: mit(func(e *Einstellung) {
		// Integralfilm: flau, warm, grünliche Schatten, helle Ecken abgedunkelt
		e.Matt, e.Temperatur, e.Kontrast, e.Saettigung, e.Lichter, e.Vignette = 0.3, 0.2, -0.1, -0.15, -0.3, 0.3
		e.TonTiefen, e.TonStaerke = [3]float64{0.45, 0.53, 0.48}, 0.4
	})},
	{Name: "Sofortbild kühl", Gruppe: "Sofortbild", Werte: mit(func(e *Einstellung) {
		// Abziehfilm: kühl, bläuliche Schatten
		e.Temperatur, e.Toenung, e.Matt, e.Kontrast, e.Saettigung, e.Vignette = -0.15, -0.05, 0.2, 0.05, -0.1, 0.2
		e.Kanal[2].Tiefe = 0.15
	})},
	{Name: "Sofortbild S/W", Gruppe: "Sofortbild", Werte: mit(func(e *Einstellung) {
		e.SW, e.Matt, e.Kontrast, e.Vignette, e.Koernung = 1, 0.25, 0.1, 0.25, 0.1
		e.TonLichter, e.TonStaerke = [3]float64{0.53, 0.51, 0.48}, 0.3
	})},
	{Name: "Kino", Gruppe: "Retro & Kino", Werte: mit(func(e *Einstellung) {
		e.TonLichter, e.TonTiefen, e.TonStaerke = [3]float64{0.85, 0.62, 0.38}, [3]float64{0.22, 0.52, 0.6}, 0.75
		e.Kontrast, e.Saettigung, e.Vignette = 0.25, -0.1, 0.25
	})},
	{Name: "Vintage", Gruppe: "Retro & Kino", Werte: mit(func(e *Einstellung) {
		e.Saettigung, e.Matt, e.Temperatur, e.Kontrast, e.Vignette, e.Koernung = -0.35, 0.45, 0.25, -0.1, 0.4, 0.4
		e.Kanal[0].Tiefe, e.Kanal[2].Licht = 0.15, -0.2
	})},
	{Name: "Matt", Gruppe: "Retro & Kino", Werte: mit(func(e *Einstellung) {
		e.Matt, e.Kontrast, e.Saettigung = 0.55, -0.1, -0.15
	})},
	{Name: "S/W klassisch", Gruppe: "Schwarzweiß", Werte: mit(func(e *Einstellung) {
		e.SW, e.Kontrast = 1, 0.15
	})},
	{Name: "S/W kontrastreich", Gruppe: "Schwarzweiß", Werte: mit(func(e *Einstellung) {
		e.SW, e.SWMix, e.Kontrast, e.Klarheit, e.Tiefen = 1, [3]float64{0.4, 0.5, 0.1}, 0.55, 0.35, -0.1
	})},
	{Name: "S/W Rotfilter", Gruppe: "Schwarzweiß", Werte: mit(func(e *Einstellung) {
		// dunkler, dramatischer Himmel wie mit Rotfilter vor dem Objektiv
		e.SW, e.SWMix, e.Kontrast, e.Klarheit = 1, [3]float64{0.95, 0.3, -0.25}, 0.3, 0.2
	})},
	{Name: "S/W weich", Gruppe: "Schwarzweiß", Werte: mit(func(e *Einstellung) {
		e.SW, e.Kontrast, e.Matt, e.Tiefen, e.Koernung = 1, -0.15, 0.3, 0.2, 0.3
	})},
	// Schwarzweißfilme
	{Name: "S/W-Film 400 körnig", Gruppe: "Schwarzweißfilm", Werte: mit(func(e *Einstellung) {
		// der Reportagefilm: kräftiger Kontrast, deutliches Korn
		e.SW, e.SWMix, e.Kontrast, e.Koernung, e.Klarheit = 1, [3]float64{0.35, 0.55, 0.10}, 0.35, 0.4, 0.15
	})},
	{Name: "S/W-Film 400 weich", Gruppe: "Schwarzweißfilm", Werte: mit(func(e *Einstellung) {
		e.SW, e.Kontrast, e.Koernung, e.Tiefen, e.Matt = 1, 0.1, 0.3, 0.1, 0.05
	})},
	{Name: "S/W-Film 100 feinkörnig", Gruppe: "Schwarzweißfilm", Werte: mit(func(e *Einstellung) {
		e.SW, e.Kontrast, e.Koernung, e.Klarheit = 1, 0.2, 0.08, 0.1
	})},
	{Name: "S/W-Film gepusht 3200", Gruppe: "Schwarzweißfilm", Werte: mit(func(e *Einstellung) {
		// zwei Stufen gepusht: harte Kontraste, grobes Korn
		e.SW, e.Kontrast, e.Koernung, e.Tiefen, e.Klarheit, e.Vignette = 1, 0.6, 0.7, -0.15, 0.2, 0.25
	})},
	{Name: "Infrarot S/W", Gruppe: "Schwarzweißfilm", Werte: mit(func(e *Einstellung) {
		// Laub hell wie Schnee, Himmel fast schwarz, leichtes Leuchten
		e.SW, e.SWMix, e.Kontrast, e.Klarheit, e.Koernung = 1, [3]float64{0.2, 1.3, -0.7}, 0.35, -0.2, 0.15
	})},
	{Name: "Orthochromatisch", Gruppe: "Schwarzweißfilm", Werte: mit(func(e *Einstellung) {
		// unempfindlich für Rot: Rottöne werden dunkel, Haut kräftig
		e.SW, e.SWMix, e.Kontrast, e.Koernung = 1, [3]float64{0.0, 0.45, 0.55}, 0.2, 0.15
	})},
	// Edeldruckverfahren und Tonungen
	{Name: "Sepia", Gruppe: "Edeldruck & Tonung", Werte: mit(func(e *Einstellung) {
		e.SW, e.TonLichter, e.TonTiefen, e.TonStaerke, e.Matt, e.Vignette = 1, [3]float64{0.78, 0.62, 0.4}, [3]float64{0.65, 0.48, 0.32}, 0.85, 0.12, 0.2
	})},
	{Name: "Selen", Gruppe: "Edeldruck & Tonung", Werte: mit(func(e *Einstellung) {
		e.SW, e.TonTiefen, e.TonLichter, e.TonStaerke, e.Kontrast = 1, [3]float64{0.58, 0.42, 0.62}, [3]float64{0.52, 0.5, 0.48}, 0.7, 0.2
	})},
	{Name: "Palladium", Gruppe: "Edeldruck & Tonung", Werte: mit(func(e *Einstellung) {
		// warmes Braunschwarz, sehr lange, weiche Tonwertskala, mattes Papier
		e.SW, e.Matt, e.Kontrast, e.Tiefen, e.Klarheit, e.Koernung, e.Vignette = 1, 0.2, -0.15, 0.1, -0.1, 0.05, 0.1
		e.TonTiefen, e.TonLichter, e.TonStaerke = [3]float64{0.68, 0.50, 0.33}, [3]float64{0.62, 0.56, 0.44}, 1
	})},
	{Name: "Platin", Gruppe: "Edeldruck & Tonung", Werte: mit(func(e *Einstellung) {
		// neutraler bis leicht warmer Grauton, weich und tief
		e.SW, e.Matt, e.Kontrast, e.Tiefen, e.Klarheit = 1, 0.15, -0.05, 0.05, -0.05
		e.TonTiefen, e.TonLichter, e.TonStaerke = [3]float64{0.55, 0.51, 0.47}, [3]float64{0.54, 0.52, 0.48}, 0.7
	})},
	{Name: "Van-Dyke-Braun", Gruppe: "Edeldruck & Tonung", Werte: mit(func(e *Einstellung) {
		// tiefes Schokoladenbraun
		e.SW, e.Kontrast, e.Matt = 1, 0.15, 0.1
		e.TonTiefen, e.TonLichter, e.TonStaerke = [3]float64{0.70, 0.48, 0.30}, [3]float64{0.62, 0.55, 0.44}, 1
	})},
	{Name: "Albumin", Gruppe: "Edeldruck & Tonung", Werte: mit(func(e *Einstellung) {
		// purpurbraune Schatten, gelbliche Lichter wie bei alten Abzügen
		e.SW, e.Kontrast, e.Matt, e.Vignette = 1, 0.1, 0.1, 0.2
		e.TonTiefen, e.TonLichter, e.TonStaerke = [3]float64{0.66, 0.44, 0.50}, [3]float64{0.66, 0.60, 0.40}, 1
	})},
	{Name: "Salzpapier", Gruppe: "Edeldruck & Tonung", Werte: mit(func(e *Einstellung) {
		// rötliches Braun, flau und weich – das älteste Papierverfahren
		e.SW, e.Kontrast, e.Matt, e.Klarheit, e.Koernung = 1, -0.25, 0.25, -0.2, 0.1
		e.TonTiefen, e.TonLichter, e.TonStaerke = [3]float64{0.74, 0.46, 0.36}, [3]float64{0.62, 0.56, 0.47}, 1
	})},
	{Name: "Cyanotypie", Gruppe: "Edeldruck & Tonung", Werte: mit(func(e *Einstellung) {
		// Preußischblau auf weißem Papier
		e.SW, e.Kontrast, e.Matt = 1, 0.1, 0.1
		e.TonTiefen, e.TonLichter, e.TonStaerke = [3]float64{0.0, 0.3, 1.0}, [3]float64{0.42, 0.5, 0.6}, 1
	})},
}
