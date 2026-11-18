// Bedienungsanleitung Tri-Vertical 3.8.5 als Word-Dokument (Design Hell)
const fs = require("fs");
const {
  Document, Packer, Paragraph, TextRun, ImageRun, Table, TableRow, TableCell, WidthType,
  AlignmentType, HeadingLevel, LevelFormat, BorderStyle, ShadingType, Footer, Header,
  PageNumber, PageBreak, TabStopType,
} = require("docx");

const FARBE = "1A1A1A";     // fast Schwarz für Überschriften
const ROT = "0F6CBD";       // Akzentfarbe (Blau wie im Design „Hell“)
const HELL = "E8EEF5";      // Tabellenköpfe
const HINWEIS = "F0F6FC";   // Hinweiskästen (hellblau)
const BREITE = 9638;        // A4 mit 2 cm Rand: 11906 − 2×1134

// ---------------------------------------------------------------- Bausteine
const text = (t, o = {}) => new TextRun({ text: t, ...o });
const fett = (t) => new TextRun({ text: t, bold: true });

function absatz(...teile) {
  return new Paragraph({ spacing: { after: 120 }, children: teile.map((t) => (typeof t === "string" ? text(t) : t)) });
}
function h1(t) { return new Paragraph({ heading: HeadingLevel.HEADING_1, pageBreakBefore: true, children: [text(t)] }); }
function h2(t) { return new Paragraph({ heading: HeadingLevel.HEADING_2, children: [text(t)] }); }
function punkt(...teile) {
  return new Paragraph({ numbering: { reference: "punkte", level: 0 }, spacing: { after: 60 },
    children: teile.map((t) => (typeof t === "string" ? text(t) : t)) });
}
let listeNr = 0;
function schritte(liste) {
  const ref = "schritte" + (++listeNr);
  schrittListen.push(ref);
  return liste.map((teile) => new Paragraph({ numbering: { reference: ref, level: 0 }, spacing: { after: 60 },
    children: (Array.isArray(teile) ? teile : [teile]).map((t) => (typeof t === "string" ? text(t) : t)) }));
}
const schrittListen = [];

function bild(datei, breitePx, unterschrift) {
  if (datei.endsWith(".png") && fs.existsSync(datei.replace(".png", ".jpg"))) datei = datei.replace(".png", ".jpg");
  const { w, h } = groesse(datei);
  const hoehe = Math.round(h * breitePx / w);
  const out = [new Paragraph({ alignment: AlignmentType.CENTER, spacing: { before: 120, after: 60 },
    children: [new ImageRun({ type: datei.endsWith(".jpg") ? "jpg" : "png", data: fs.readFileSync(datei), transformation: { width: breitePx, height: hoehe },
      altText: { title: unterschrift, description: unterschrift, name: datei } })] })];
  if (unterschrift) out.push(new Paragraph({ alignment: AlignmentType.CENTER, spacing: { after: 200 },
    children: [text(unterschrift, { italics: true, size: 18, color: "555555" })] }));
  return out;
}
function groesse(datei) { const p = datei.replace(".jpg", ".png"); const b = fs.readFileSync(p); return { w: b.readUInt32BE(16), h: b.readUInt32BE(20) }; }

const rand = { style: BorderStyle.SINGLE, size: 4, color: "C8C8C8" };
const raender = { top: rand, bottom: rand, left: rand, right: rand };

function tabelle(kopf, zeilen, breiten) {
  const summe = breiten.reduce((a, b) => a + b, 0);
  const zelle = (inhalt, i, istKopf) => new TableCell({
    borders: raender, width: { size: breiten[i], type: WidthType.DXA },
    shading: istKopf ? { fill: HELL, type: ShadingType.CLEAR, color: "auto" } : undefined,
    margins: { top: 60, bottom: 60, left: 100, right: 100 },
    children: [new Paragraph({ children: [istKopf ? fett(inhalt) : text(inhalt)] })],
  });
  return new Table({
    width: { size: summe, type: WidthType.DXA }, columnWidths: breiten,
    rows: [new TableRow({ tableHeader: true, children: kopf.map((k, i) => zelle(k, i, true)) }),
      ...zeilen.map((z) => new TableRow({ children: z.map((c, i) => zelle(c, i, false)) }))],
  });
}

function kasten(titel, ...zeilen) {
  return new Table({
    width: { size: BREITE, type: WidthType.DXA }, columnWidths: [BREITE],
    rows: [new TableRow({ cantSplit: true, children: [new TableCell({
      borders: { top: { style: BorderStyle.SINGLE, size: 4, color: "D0D0D0" }, bottom: { style: BorderStyle.SINGLE, size: 4, color: "D0D0D0" },
        left: { style: BorderStyle.SINGLE, size: 24, color: ROT }, right: { style: BorderStyle.SINGLE, size: 4, color: "D0D0D0" } },
      width: { size: BREITE, type: WidthType.DXA }, shading: { fill: HINWEIS, type: ShadingType.CLEAR, color: "auto" },
      margins: { top: 100, bottom: 100, left: 160, right: 160 },
      children: [new Paragraph({ spacing: { after: 60 }, children: [fett(titel)] }),
        ...zeilen.map((z) => new Paragraph({ spacing: { after: 40 }, children: [text(z)] }))],
    })] })],
  });
}
const luecke = () => new Paragraph({ spacing: { after: 120 }, children: [] });


// ---------------------------------------------------------------- Inhalt (Stand 3.8)
const inhalt = [];
const B = "c/";
const ohneRand = { top: { style: BorderStyle.NONE }, bottom: { style: BorderStyle.NONE }, left: { style: BorderStyle.NONE }, right: { style: BorderStyle.NONE } };
function nebeneinander(links, rechts, bl = 4819) {
  return new Table({ width: { size: BREITE, type: WidthType.DXA }, columnWidths: [bl, BREITE - bl],
    rows: [new TableRow({ children: [
      new TableCell({ borders: ohneRand, width: { size: bl, type: WidthType.DXA }, margins: { right: 120 }, children: links }),
      new TableCell({ borders: ohneRand, width: { size: BREITE - bl, type: WidthType.DXA }, margins: { left: 120 }, children: rechts }),
    ] })] });
}

// Titelseite: nur die Oberfläche, ohne Bild
inhalt.push(
  new Paragraph({ spacing: { before: 1000, after: 160 }, indent: { left: 300 },
    children: [new ImageRun({ type: "png", data: fs.readFileSync(B + "logo.png"), transformation: { width: 300, height: Math.round(300 * groesse(B + "logo.png").h / groesse(B + "logo.png").w) },
      altText: { title: "Tri-Vertical", description: "Logo Tri-Vertical", name: "logo" } })] }),
  new Paragraph({ spacing: { after: 80 }, indent: { left: 300 },
    children: [text("Perspektivkorrektur in drei Achsen für Fotos unter Windows", { size: 30, color: "333333" })] }),
  new Paragraph({ spacing: { after: 500 }, indent: { left: 300 },
    children: [text("BEDIENUNGSANLEITUNG  ·  VERSION 3.8.5  ·  OKTOBER 2026", { size: 20, color: "777777" })] }),
  ...bild(B + "leer.png", 640, ""),
  new Paragraph({ alignment: AlignmentType.CENTER, spacing: { before: 400 },
    children: [text("Mit den Zusatzprogrammen Drucken, Rahmen, Filter, KI-Füllen und Kameraneigung.", { size: 20, color: "444444" })] }),
  new Paragraph({ alignment: AlignmentType.CENTER, spacing: { before: 80 },
    children: [text("Nur für den privaten Gebrauch.", { italics: true, size: 20, color: "666666" })] }),
);

// Inhaltsübersicht
const kapitel = ["Überblick: Funktionen und Zusatzprogramme", "Installation und Deinstallation", "Die Programmoberfläche",
  "Bilder korrigieren", "Ausgabegröße und Druckqualität", "RAW-Dateien (DNG)", "Kameras und Kameraprofile", "Zusatzprogramme",
  "Updates", "Tastenkürzel und Maus", "Problemlösung"];
inhalt.push(new Paragraph({ heading: HeadingLevel.HEADING_1, pageBreakBefore: true, children: [text("Inhalt")] }));
kapitel.forEach((k, i) => inhalt.push(new Paragraph({ spacing: { after: 100 },
  children: [text(`${i + 1}.`, { bold: true, color: ROT }), text("\t" + k)],
  tabStops: [{ type: TabStopType.LEFT, position: 500 }] })));
inhalt.push(luecke(), kasten("Neu seit Version 2.8.1",
  "• Bildausschnitt für Fotopapier und feste Pixelgrößen: Rahmen verschieben und skalieren – auch größer als das Foto (Rand in der Randfarbe) –, Vorschlag des passendsten Papierformats; auf Wunsch gleich weiterbearbeiten (Kapitel 5.4 und 5.5).",
  "• Neuer Name: Tri-Vertical heißt ab Version 3.7 Tri-Vertical – mit eigenem Logo, das sich den drei Designs anpasst, und neuem Programmsymbol (Kapitel 2.2).",
  "• Neue Automatik (Taste A): wertet die Kantenrichtungen des ganzen Bildes aus und richtet danach aus – auch Innenräume, Holzhäuser und Hochhäuser (Kapitel 4.2).",
  "• Automatisch frontal (Umschalt+A): richtet zusätzlich den Yaw aus – eine Fassade wird frontal (Kapitel 4.3).",
  "• Drehen in 90°-Schritten und Spiegeln (Kapitel 4.5).",
  "• Ausgabegröße mit Standard- und eigenen Größen, alle üblichen Fotopapier-Formate und Anzeige der Druckqualität in dpi (Kapitel 5).",
  "• Drittes Design „Analog“; nach einer Neuinstallation startet Tri-Vertical im Design „Hell“ (Kapitel 3.4).",
  "• Neue Zusatzprogramme: Rahmen (eigenständig) und Kameraneigung – die Korrektur mit den Neigungsdaten der Kamera ist jetzt ein Zusatz (Kapitel 8).",
  "• Zusatzprogramme direkt in Tri-Vertical entfernen: Hilfe → Zusatzprogramme (Kapitel 2.5)."));

// 1 Überblick
inhalt.push(h1("1. Überblick: Funktionen und Zusatzprogramme"),
  absatz("Tri-Vertical richtet Fotos perspektivisch aus: Stürzende Linien werden wieder senkrecht, ein schiefer Horizont wird gerade. Das Programm rechnet dazu die Kamera virtuell in die Waagerechte zurück – so wie es eine Fachkamera oder ein Shift-Objektiv beim Fotografieren tun würde. Weil eine echte Kameradrehung nachgebildet wird, bleiben die Proportionen erhalten."),
  tabelle(["Achse", "Bedeutung", "Typische Wirkung"], [
    ["Roll", "Drehung der Kamera um die Blickrichtung", "Horizont gerade richten"],
    ["Pitch", "Neigung der Kamera nach oben oder unten", "Stürzende Linien aufrichten (Gebäude, Innenräume)"],
    ["Yaw", "Drehung nach links oder rechts", "Fläche frontal stellen, virtueller Perspektivwechsel"],
  ], [1400, 3800, 4438]),
  h2("1.1 Die Funktionen von Tri-Vertical"),
  tabelle(["Funktion", "Kurz erklärt", "Kapitel"], [
    ["Automatisch ausrichten", "Senkrechte im Bild werden gerade – ein Tastendruck (A)", "4.2"],
    ["Automatisch frontal", "zusätzlich Waagerechte gerade, Fläche wird frontal (Umschalt+A)", "4.3"],
    ["Regler Roll, Pitch, Yaw, Brennweite", "Feineinstellung per Maus, Mausrad oder Zahleneingabe", "4.4"],
    ["Drehen und Spiegeln", "90° links/rechts, waagerecht/senkrecht spiegeln", "4.5"],
    ["Vorher / Nachher, Original, Raster, Zoom", "Ergebnis prüfen und vergleichen, Zoom bis 400 %", "4.6"],
    ["Zuschneiden oder Randfarbe", "größtes Rechteck ohne Rand – oder ganzes Bild mit Rand", "4.7"],
    ["Ausgabegröße", "Standardgrößen, Fotopapier-Formate, eigene gespeicherte Größen", "5"],
    ["Druckqualität", "zeigt, wie viele dpi das Bild auf dem gewählten Papier hat", "5.3"],
    ["Speichern", "JPEG, TIFF oder PNG; Kameradaten und Farbprofil bleiben erhalten", "4.8"],
    ["RAW-Entwicklung (DNG)", "DNG-Dateien direkt öffnen", "6"],
    ["Kameraprofile", "Kamera und Kleinbild-Brennweite erkennen", "7"],
    ["Designs Hell, Dunkel, Analog", "drei Gestaltungen der Oberfläche", "3.4"],
    ["Updates", "kleine .v3update-Datei per Doppelklick, rückgängig machbar", "9"],
  ], [3200, 5138, 1300]),
  h2("1.2 Die Zusatzprogramme"),
  absatz("Zusatzprogramme erweitern Tri-Vertical. Jedes hat ein eigenes Setup und lässt sich unabhängig installieren und wieder entfernen. Sie übernehmen das geöffnete Foto samt Korrektur, Drehung und Spiegelung direkt aus Tri-Vertical."),
  tabelle(["Zusatzprogramm", "Wofür?", "Aufruf", "Setup-Datei"], [
    ["Drucken 2.2", "Drucken mit Papierformat, Bildgröße und Kopien", "Strg+P", "Tri-Vertical-Drucken-Setup-2.2.exe"],
    ["Rahmen 1.5", "19 Rahmen mit Beschriftung; gerahmtes Bild speichern oder drucken", "Strg+R", "Tri-Vertical-Rahmen-Setup-1.5.exe"],
    ["Filter 1.5", "55 Bildlooks: Farbe, Kamera-Looks, Farbnegativ, Dia, Sofortbild, Schwarzweiß, Edeldruck (u. a. Palladium)", "Strg+F", "Tri-Vertical-Filter-Setup-1.5.exe"],
    ["KI-Füllen 1.2", "leere Ränder nach der Korrektur per KI füllen – Originalformat bleibt", "Strg+K", "Tri-Vertical-KI-Fuellen-Setup-1.2.exe"],
    ["Kameraneigung 1.3", "Neigungswinkel aus der Bilddatei übernehmen; Stapelverarbeitung ganzer Ordner", "Menü Datei", "Tri-Vertical-Kameraneigung-Setup-1.3.exe"],
  ], [1900, 4100, 1200, 2438]),
);

// 2 Installation
inhalt.push(h1("2. Installation und Deinstallation"),
  h2("2.1 Voraussetzungen"),
  punkt("Windows 10 oder Windows 11 (64 Bit)"),
  punkt("etwa 15 MB freier Speicherplatz für Tri-Vertical; KI-Füllen braucht zusätzlich rund 120 MB"),
  punkt("keine Administratorrechte; Internet nur für das einmalige Laden des KI-Modells und für die Online-KI"),
  h2("2.2 Tri-Vertical installieren"),
  ...schritte([
    ["Laufendes Tri-Vertical schließen und Doppelklick auf ", fett("Tri-Vertical-Setup-3.8.3.exe"), "."],
    ["Erscheint „Der Computer wurde durch Windows geschützt“: auf ", fett("Weitere Informationen"), " und dann auf ", fett("Trotzdem ausführen"), " klicken. Die Meldung kommt, weil das Setup nicht kostenpflichtig signiert ist."],
    "Dem Assistenten folgen. Tri-Vertical erscheint danach auf dem Desktop und im Startmenü (Ordner „Vertical3“).",
  ]),
  kasten("Neuer Name", "Tri-Vertical ist das bisherige Vertical3. Setups, Updates, Startmenü und alle Zusatzprogramme heißen jetzt Tri-Vertical und tragen das neue Logo. Nur intern – Programmdatei (Vertical3.exe), Programmordner und Einstellungen – bleibt der alte Name, damit Updates, Einstellungen, Kameraprofile und Zusatzprogramme ohne Neuinstallation weiterlaufen."),
  absatz("Nach einer Neuinstallation startet Tri-Vertical im Design „Hell“. Läuft das Setup über eine bestehende Installation, bleiben gewähltes Design, eigene Größen und Kameraprofile erhalten."),
  h2("2.3 Zusatzprogramme installieren"),
  ...schritte([
    "Tri-Vertical schließen.",
    ["Doppelklick auf das Setup des Zusatzprogramms, z. B. ", fett("Tri-Vertical-Rahmen-Setup-1.5.exe"), ". Es findet den Programmordner von Tri-Vertical selbst."],
    ["Tri-Vertical starten. Das Zusatzprogramm erscheint im Menü ", fett("Datei"), " und als Knopf unten im Bedienfeld."],
  ]),
  absatz("Eine neuere Version eines Zusatzprogramms wird einfach über die alte installiert."),
  h2("2.4 Tri-Vertical deinstallieren"),
  absatz("Windows-Einstellungen → Apps → Installierte Apps → „Tri-Vertical“. Dabei werden alle Zusatzprogramme mit entfernt. Ihre Fotos und gespeicherten Ergebnisse bleiben immer unberührt."),
  h2("2.5 Zusatzprogramme entfernen"),
);
inhalt.push(nebeneinander(bild(B + "zusaetze.png", 300, "Hilfe → Zusatzprogramme"), [
  absatz("Am einfachsten direkt in Tri-Vertical:"),
  ...schritte([
    "Das Zusatzprogramm schließen, falls es geöffnet ist.",
    [fett("Hilfe → Zusatzprogramme …"), " wählen."],
    ["Den Zusatz anklicken und ", fett("Ausgewähltes Zusatzprogramm entfernen …"), "."],
    "Tri-Vertical neu starten.",
  ]),
  absatz("Entfernt werden Programmdatei, Kurzanleitung, Startmenü-Eintrag und der Eintrag in den Windows-Einstellungen."),
], 4900));

// 3 Oberfläche
inhalt.push(h1("3. Die Programmoberfläche"),
  absatz("Links liegt die Bildfläche, rechts das Bedienfeld, unten die Statuszeile, oben die Menüleiste. Die Abbildungen zeigen das Design „Hell“."),
  ...bild(B + "haupt.png", 620, "Hauptfenster: Foto automatisch ausgerichtet – grüne Punkte zeigen die Kanten, die jetzt gerade stehen"),
  h2("3.1 Das Bedienfeld"),
  nebeneinander([...bild(B + "panel_oben.png", 150, "oberer Teil"), ...[]], [
    ...[["Bild öffnen", "Foto auswählen (auch Strg+O)."],
      ["Bildinformationen", "Dateiname, Größe, Kamera und Profil; mit dem Zusatz Kameraneigung auch die Neigungsdaten."],
      ["Drehen und Spiegeln", "90° nach links oder rechts drehen, waagerecht oder senkrecht spiegeln."],
      ["Korrektur", "Regler für Roll, Pitch, Yaw und Brennweite. „Kameraneigung“ (nur mit Zusatz) stellt die Kamerawerte wieder her, „Zurücksetzen“ setzt alles auf 0."],
      ["Ausrichten", "„Automatisch ausrichten“ (A) und „Automatisch frontal“ (Umschalt+A)."]]
      .map(([t, b]) => new Paragraph({ spacing: { after: 120 }, children: [fett(t + ": "), text(b)] })),
  ], 3000),
  luecke(),
  nebeneinander([...bild(B + "panel_unten.png", 150, "unterer Teil (gescrollt)")], [
    ...[["Ansicht", "Schalter für Zuschneiden, Hilfsraster, erkannte Kanten, Vorher/Nachher und Original – blau heißt eingeschaltet. Darunter Zoom und Randfarbe."],
      ["Größe", "Ausgabegröße für das Speichern; bei Fotopapier steht darunter die Druckqualität (Kapitel 5)."],
      ["Speichern", "Blauer Knopf (Strg+S)."],
      ["Zusatzprogramme", "Je installiertem Zusatz ein Knopf: Drucken, Filter, KI-Füllen, Stapelverarbeitung, Rahmen."]]
      .map(([t, b]) => new Paragraph({ spacing: { after: 120 }, children: [fett(t + ": "), text(b)] })),
  ], 3000),
  luecke(),
  kasten("Ausgegraute Knöpfe und Scrollen", "Knöpfe in grauer Schrift sind gerade nicht verfügbar – z. B. alles Bildbezogene, solange kein Bild geöffnet ist. Passt das Bedienfeld nicht ganz auf den Bildschirm, mit dem Mausrad über dem Bedienfeld scrollen."),
  h2("3.2 Die Menüs"),
  nebeneinander(bild(B + "m_datei.png", 230, "Menü Datei (mit allen Zusatzprogrammen)"), bild(B + "m_ansicht.png", 200, "Menü Ansicht")),
  nebeneinander(bild(B + "m_bild.png", 180, "Menü Bild"), bild(B + "m_hilfe.png", 180, "Menü Hilfe")),
  tabelle(["Menü", "Inhalt"], [
    ["Datei", "Bild öffnen, Speichern unter, die installierten Zusatzprogramme, Beenden"],
    ["Kameras", "Kameraprofil importieren, bekannte Kameras anzeigen, Profilordner öffnen"],
    ["Bild", "Nach links / rechts drehen, waagerecht / senkrecht spiegeln, Ausgabegröße"],
    ["Ansicht", "Vorher/Nachher, Original, Hilfsraster, erkannte Kanten, Design, Zoom, Automatisch ausrichten / frontal"],
    ["Hilfe", "Anleitung, Update installieren, Zusatzprogramme (mit Entfernen), letztes Update rückgängig machen, Über Tri-Vertical"],
  ], [1800, 7838]),
  h2("3.3 Statuszeile"),
  absatz("Unten stehen Hinweise und Ergebnisse – z. B. die gefundenen Winkel und die Übereinstimmung der Automatik – rechts die Zoomstufe."),
  h2("3.4 Design: Hell, Dunkel oder Analog"),
  absatz("Unter ", fett("Ansicht → Design"), " stehen drei Gestaltungen zur Wahl. Die Auswahl wirkt sofort und wird gespeichert. Nach einer Neuinstallation startet Tri-Vertical immer im Design „Hell“."),
  nebeneinander(bild(B + "dunkel.png", 300, "Design „Dunkel“"), bild(B + "analog.png", 300, "Design „Analog“")),
  tabelle(["Design", "Aussehen"], [
    ["Hell (Standard)", "helle Flächen, blaue Akzente, Schrift Segoe UI – im Stil von Microsoft Office"],
    ["Dunkel", "dunkle Flächen wie ein Kameramenü, weiße Schrift, rote Akzente – das Foto wirkt besonders kräftig"],
    ["Analog", "warm wie Fotopapier und Dunkelkammer: Elfenbein, Sepia-Schrift, Rotlicht-Akzent, Werte in Schreibmaschinenschrift"],
  ], [2400, 7238]),
);

// 4 Bilder korrigieren
inhalt.push(h1("4. Bilder korrigieren"),
  h2("4.1 Bild öffnen"),
  absatz("Über ", fett("Bild öffnen"), ", das Menü Datei oder Strg+O. Alternativ ein Foto im Explorer mit der rechten Maustaste anklicken → ", fett("Öffnen mit → Tri-Vertical"), "."),
  tabelle(["Format", "Endung", "Typische Quelle"], [
    ["JPEG", ".jpg, .jpeg", "fast alle Kameras und Smartphones"],
    ["HEIC / HEIF", ".heic, .heif, .hif", "iPhone, Smartphones; .HIF von Canon, Fujifilm, Sony"],
    ["DNG (RAW)", ".dng", "Leica, iPhone ProRAW, Pentax, Ricoh, Adobe DNG Converter"],
    ["TIFF", ".tif, .tiff", "Exporte aus Lightroom, Photoshop, Scanner"],
    ["PNG / WebP", ".png, .webp", "Bildschirmfotos, Grafiken, Bilder aus dem Internet"],
  ], [1800, 2400, 5438]),
  h2("4.2 Automatisch ausrichten (Taste A)"),
  absatz("Ein Klick auf ", fett("Automatisch ausrichten"), " oder die Taste ", fett("A"), " genügt. Tri-Vertical bestimmt für jeden Bildpunkt mit deutlicher Kante die örtliche Richtung und sucht dann die Kamerawinkel, bei denen nach der Korrektur möglichst viel davon genau senkrecht steht – eine Statistik über das ganze Bild. Das funktioniert bei Gebäuden, Innenräumen, Möbeln, Holzhäusern und Messeständen."),
  h2("Vorher – nachher"),
  ...bild(B + "vergleich.png", 620, "Ansicht Vorher/Nachher (Taste V): links das Original mit stürzenden Linien, rechts automatisch ausgerichtet"),
  punkt(fett("Grüne Punkte"), " zeigen die Kanten, die nach der Korrektur gerade stehen. Ein- und ausblenden mit dem Schalter ", fett("Erkannte Kanten anzeigen"), " oder der Taste ", fett("K"), "."),
  punkt("Die Statuszeile nennt die Winkel und die ", fett("Übereinstimmung"), " in Prozent. Ist das Ergebnis unsicher, steht dort „bitte prüfen“."),
  punkt("Ohne senkrechte Strukturen (Meer, Strand) dreht Tri-Vertical nur nach dem Horizont. Ohne jede klare Struktur (Wald, Wiese, Porträt) meldet es das und ändert nichts."),
  punkt("Sehr starke Korrekturen – etwa ein steil nach oben fotografiertes Hochhaus – wendet die Automatik nur so weit an, dass das Bild nicht extrem verzerrt wird. Die Statuszeile nennt den vollen Wert; am Regler „Pitch“ lässt sich weiter korrigieren."),
  kasten("Wichtig: die Brennweite", "Für einen genauen Pitch muss die Brennweite stimmen. Tri-Vertical liest sie aus den Kameradaten. Fehlen diese (z. B. bei Bildern aus Messengern oder dem Internet), rechnet es mit 28 mm – dann den Regler „Brennweite“ auf den richtigen Wert stellen und erneut A drücken."),
  h2("4.3 Automatisch frontal (Umschalt+A)"),
  kasten("Yaw-Achse ist standardmäßig ausgeschaltet", "Yaw-Regler, „Automatisch frontal“ und Umschalt+A erscheinen erst, wenn im Menü Ansicht → „Yaw-Achse verwenden“ eingeschaltet ist. Die Einstellung bleibt gespeichert. Ausgeschaltet korrigiert Tri-Vertical nur Roll und Pitch – für stürzende Linien und schiefe Horizonte genügt das."),
  absatz("Wertet zusätzlich die waagerechten Kanten aus und bestimmt so auch den Yaw: Eine Fassade, Wand oder Tafel wird dargestellt, als stünde man genau davor. Nur für frontal gemeinte Motive – bei Gebäuden über Eck wird das Bild sonst stark verzerrt."),
  h2("4.4 Die Regler"),
  tabelle(["Regler", "Wirkung", "Hinweis"], [
    ["Roll", "Bild drehen, Horizont gerade richten", "−45° bis +45°"],
    ["Pitch", "Stürzende Linien aufrichten", "positiv = Kamera war nach oben geneigt"],
    ["Yaw", "Fläche frontal stellen, Perspektivwechsel", "meist 0, außer bei „frontal“; Bereich passend zur Brennweite begrenzt (z. B. ±27,5° bei 23 mm quer), steht als „max ±…“ neben dem Regler"],
    ["Brennweite", "Kleinbild-äquivalente Brennweite", "wird aus dem Foto gelesen; wichtig für einen genauen Pitch"],
  ], [1600, 3800, 4238]),
  luecke(),
  punkt(fett("Ziehen: "), "den Knopf mit der Maus verschieben."),
  punkt(fett("Mausrad: "), "Maus über den Regler halten und drehen – 0,1° je Raste, mit gedrückter Umschalt-Taste 1°."),
  punkt(fett("Eintippen: "), "auf die Zahl rechts neben dem Namen klicken, Wert eingeben, OK."),
  h2("4.5 Drehen und Spiegeln"),
  absatz("Im Bedienfeld oder im Menü ", fett("Bild"), ": ", fett("Links drehen 90°"), " (Strg+←), ", fett("Rechts drehen 90°"), " (Strg+→), ", fett("Spiegeln ↔"), " (links/rechts) und ", fett("Spiegeln ↕"), " (oben/unten). Die Korrekturwerte werden automatisch mitgedreht – eine bereits gemachte Korrektur bleibt erhalten. Am besten ein falsch liegendes Bild zuerst drehen und danach automatisch ausrichten."),
  h2("4.6 Ansicht und Zoom"),
  punkt(fett("Vorher / Nachher"), " (Taste V): Original und Ergebnis im gleichen Maßstab nebeneinander."),
  punkt(fett("Original zeigen"), " (Leertaste): schnelles Umschalten zwischen Original und Ergebnis."),
  punkt(fett("Hilfsraster"), ": Linien zum Prüfen, ob Kanten wirklich senkrecht stehen."),
  tabelle(["Zoom", "So geht's"], [
    ["Vergrößern / verkleinern", "Mausrad über dem Bild (vergrößert dort, wo die Maus steht), Tasten + und −, Knöpfe − / + im Bedienfeld"],
    ["Ganzes Bild / 100 %", "Taste 0 bzw. Taste 1 (ein Bildschirmpunkt = ein Fotopixel)"],
    ["Ausschnitt verschieben", "im vergrößerten Bild mit gedrückter linker Maustaste ziehen"],
  ], [3200, 6438]),
  h2("4.7 Zuschneiden, Randfarbe oder KI-Füllen"),
  absatz("Durch die Korrektur entstehen an den Bildrändern leere Flächen. Es gibt drei Möglichkeiten:"),
  punkt(fett("Auf Bildinhalt zuschneiden"), " (Standard): das größtmögliche Rechteck ohne Rand."),
  punkt(fett("Randfarbe"), ": Ist der Schalter aus, bleibt das ganze Bild erhalten; die Ränder werden weiß, schwarz oder grau."),
  punkt(fett("KI-Füllen"), " (Zusatzprogramm, Kapitel 8.5): Das Bild behält das Format des Originals, die Ränder füllt eine KI passend zum Motiv."),
  h2("4.8 Speichern"),
  absatz(fett("Speichern"), " (Strg+S) speichert das Ergebnis in der gewählten Ausgabegröße (Kapitel 5). Vorgeschlagen wird der Originalname mit dem Zusatz „_korr“. Das Original bleibt immer unverändert. HEIC-, HIF- und DNG-Fotos werden standardmäßig als JPEG gespeichert."),
  tabelle(["Format", "Eigenschaften"], [
    ["JPEG", "Standard; Kameradaten (EXIF) und Farbprofil werden übernommen"],
    ["TIFF", "verlustfrei – ideal zur Weiterbearbeitung in Photoshop"],
    ["PNG", "verlustfrei"],
  ], [1800, 7838]),
);

// 5 Ausgabegröße
inhalt.push(h1("5. Ausgabegröße und Druckqualität"),
  absatz("Knopf ", fett("Größe"), " über „Speichern“ oder Menü ", fett("Bild → Ausgabegröße …"), ". Bei Größen ohne festes Seitenverhältnis (Original, längste Seite, Prozent) bleibt das ganze Bild erhalten; bei Fotopapier und „passt in Breite × Höhe“ legen Sie den Ausschnitt mit einem Rahmen fest (Kapitel 5.4). Die gewählte Größe gilt für alle weiteren Bilder – auch nach einem Neustart."),
  ...bild(B + "groesse.png", 420, "Fenster „Ausgabegröße“ mit Anzeige der Druckqualität"),
  h2("5.1 Standardgrößen"),
  tabelle(["Gruppe", "Größen"], [
    ["Original", "volle Auflösung"],
    ["Bildschirm", "4K UHD (passt in 3840 × 2160), Full HD (passt in 1920 × 1080)"],
    ["Längste Seite", "3000, 2048, 1600 (Web), 1080 (soziale Netzwerke), 800 px (E-Mail)"],
    ["Fotolabor", "9×13, 10×15, 11×15, 13×18, 13×19, 15×20, 15×21, 18×24, 18×27, 20×25, 20×30, 24×30, 24×36, 30×40, 30×45, 40×50, 40×60, 50×60, 50×70, 50×75, 60×80, 60×90 cm"],
    ["Quadratisch / Panorama", "10×10, 13×13, 15×15, 20×20, 30×30 cm · 10×30, 20×60 cm"],
    ["DIN", "A6, A5, A4, A3, A3+, A2"],
    ["Zoll", "4×6, 5×7, 8×10, Letter 8,5×11, 11×14, 13×19"],
    ["Prozent", "50 %, 25 %"],
  ], [2400, 7238]),
  luecke(),
  absatz("Fotopapier-Formate werden mit 300 dpi berechnet. Das gespeicherte Bild hat genau das Seitenverhältnis des Papiers; welcher Teil des Fotos verwendet wird, bestimmt der Ausschnittrahmen (Kapitel 5.4). „Kleinere Bilder nicht vergrößern“ (Standard) verhindert, dass kleine Bilder hochgerechnet werden."),
  h2("5.2 Eigene Größen"),
  ...schritte([
    ["Unten im Fenster unter ", fett("Eigene Größe anlegen"), " einen Namen eingeben, z. B. „Fotobuch“."],
    "Art wählen: längste Seite, Breite, Höhe, „passt in Breite × Höhe“, Druckformat in cm oder Prozent – und die Werte eintragen.",
    [fett("Speichern"), ". Die Größe steht danach dauerhaft in der Liste, gekennzeichnet mit „(eigene)“, und lässt sich mit „Ausgewählte eigene Größe löschen“ wieder entfernen."],
  ]),
  h2("5.3 Druckqualität"),
  absatz("Bei Fotopapier zeigt Tri-Vertical, wie viele dpi das Bild auf dem gewählten Papier tatsächlich hat – im Fenster „Ausgabegröße“ mit den für 300 dpi nötigen Pixeln und kurz im Bedienfeld unter „Größe“. Es zählen die Pixel des korrigierten Bildes; Hochrechnen bringt keine echte Schärfe."),
  tabelle(["dpi", "Bewertung"], [
    ["ab 300", "sehr gut"], ["250–299", "gut"], ["200–249", "befriedigend"], ["150–199", "ausreichend"], ["unter 150", "mangelhaft"],
  ], [3000, 6638]),
  luecke(),
  absatz("Die Schwellen gelten für jede Papiergröße. Bei Pixelgrößen (z. B. Full HD) steht dort, bis zu welcher Papiergröße das Bild sehr gut (300 dpi) bzw. gut (250 dpi) reicht. Wird nur ein Ausschnitt verwendet, zählen dessen Pixel – ein kleiner Ausschnitt senkt also die Druckqualität."),
  h2("5.4 Bildausschnitt festlegen"),
  absatz("Ist ein Fotopapier-Format (z. B. 40 × 50 cm) oder eine Größe „passt in Breite × Höhe“ gewählt, liegt auf dem korrigierten Bild ein weißer Rahmen im Seitenverhältnis dieser Größe. Der Bereich außerhalb ist abgedunkelt – er wird nicht gespeichert."),
  ...bild(B + "ausschnitt.png", 560, "Ausschnitt für 40 × 50 cm: verkleinert und nach unten verschoben"),
  ...schritte([
    [fett("Verschieben: "), "in den Rahmen klicken und ziehen."],
    [fett("Größer/kleiner: "), "an einem der vier Eckgriffe ziehen; die gegenüberliegende Ecke bleibt stehen, das Seitenverhältnis bleibt erhalten. Der Rahmen darf auch größer als das Foto werden – die überstehenden Flächen erhalten die eingestellte Randfarbe (Weiß, Schwarz oder Grau)."],
    [fett("Hoch / Quer tauschen"), " (Bedienfeld unter „Größe“) dreht den Rahmen, z. B. 50 × 40 cm statt 40 × 50 cm aus einem Hochformat-Foto."],
    [fett("Größter Ausschnitt"), " setzt den Rahmen auf die größtmögliche Fläche innerhalb des Fotos (ohne Rand) in der Bildmitte zurück."],
    [fett("Ganzes Foto einpassen (mit Rand)"), " vergrößert den Rahmen so weit, dass das ganze Foto hineinpasst – nichts wird abgeschnitten, die freien Streifen bekommen die Randfarbe."],
    [fett("Passt besser: …"), " erscheint bei Fotopapier, wenn ein anderes Format deutlich weniger abschneidet (z. B. 40 × 60 cm statt 40 × 50 cm für ein 3:2-Foto). Ein Klick übernimmt dieses Format."],
  ]),
  absatz("Der Rahmen richtet sich automatisch nach dem Bild: Hochformat-Foto → Hochformat-Rahmen, Querformat-Foto → Querformat-Rahmen. Das gilt auch nach „Links/Rechts drehen 90°“ und bei jedem neuen Bild; „Hoch / Quer tauschen“ wirkt nur für das aktuelle Bild."),
  h2("5.5 Ausschnitt speichern und weiterbearbeiten"),
  absatz("Der Knopf ", fett("Ausschnitt speichern und weiterbearbeiten"), " (unter „Hoch / Quer tauschen“) speichert den Ausschnitt in der Zielgröße – vorgeschlagen wird z. B. „Foto_korr_40x50cm.jpg“ – und öffnet die gespeicherte Datei sofort wieder in Tri-Vertical. Danach lässt sie sich wie jedes andere Bild weiterbearbeiten, etwa mit ", fett("Filter"), ", ", fett("Rahmen"), " oder ", fett("Drucken"), ". Die Korrektur steckt bereits im Bild; die Regler stehen deshalb wieder auf 0."),
  kasten("Hinweis zur Druckqualität", "Nach dem Weiterbearbeiten zeigt die Druckqualität die Pixel der neuen Datei. Wurde ein kleiner Ausschnitt hochgerechnet, steht dort „sehr gut“, obwohl die Bilddetails vom kleineren Ausschnitt stammen. Maßgeblich ist die Anzeige vor dem Speichern."),
  absatz("Beim Speichern wird genau dieser Bereich ausgeschnitten und dann auf die Zielgröße gebracht – bei 40 × 50 cm also auf 4724 × 5906 Pixel (300 dpi). Ist „Kleinere Bilder nicht vergrößern“ eingeschaltet und der Ausschnitt hat weniger Pixel, bleibt er in seiner Größe. Die Beschriftung oben im Rahmen zeigt die Pixel des Ausschnitts. Bei jedem neuen Bild beginnt der Rahmen wieder mittig in voller Größe. Der Rahmen erscheint nur in der Einzelansicht des korrigierten Bildes (nicht bei Vorher/Nachher oder „Original zeigen“)."),
);

// 6 RAW
inhalt.push(h1("6. RAW-Dateien (DNG)"),
  absatz("Tri-Vertical entwickelt RAW-Dateien im DNG-Format beim Öffnen selbst: Rohdaten entpacken, Farben aus dem Sensormuster berechnen, Weißabgleich und Farbumrechnung wie von der Kamera vorgesehen, Belichtung nach Kameravorgabe."),
  tabelle(["Unterstützt", "Nicht unterstützt"], [
    ["DNG von Leica, Pentax, Ricoh, Xiaomi u. a.", "andere RAW-Formate (NEF, CR3, CR2, RW2, ARW, RAF, ORF …) – vorher mit dem kostenlosen Adobe DNG Converter umwandeln oder JPEG/HIF verwenden"],
    ["iPhone ProRAW (ältere Kompression)", "neuere ProRAW-Dateien mit JPEG-XL-Kompression – Tri-Vertical zeigt dann das eingebettete Vorschaubild"],
    ["mit Adobe DNG Converter erzeugte DNGs", "Fujifilm-X-Trans-Sensormuster"],
  ], [4600, 5038]),
  luecke(),
  kasten("Datei heißt .dng, ist aber keine RAW-Datei?", "Manche Übertragungswege vom Handy (z. B. „Smartphone-Link“) liefern unter dem RAW-Dateinamen nur ein kleines JPEG. Tri-Vertical öffnet es trotzdem und weist darauf hin. Echte RAW-Dateien sind meist über 10 MB groß – am sichersten per USB-Kabel kopieren."),
);

// 7 Kameras
inhalt.push(h1("7. Kameras und Kameraprofile"),
  absatz("Für jeden Hersteller gibt es ein Kameraprofil. Tri-Vertical erkennt damit die Kamera und – wichtig für eine verzerrungsfreie Korrektur – die Kleinbild-Brennweite. Erkannt wird am ", fett("Hersteller"), "; andere Modelle desselben Herstellers funktionieren ebenfalls. Menü ", fett("Kameras → Bekannte Kameras anzeigen"), " zeigt alle Profile."),
  ...bild(B + "kameras.png", 600, "Übersicht der bekannten Kameras"),
  kasten("Warum die Brennweite so wichtig ist", "Viele Kameras schreiben die Kleinbild-Brennweite nicht ins Foto. Tri-Vertical rechnet sie dann aus der Sensorgröße um: APS-C × 1,5 (Canon × 1,6), Micro Four Thirds × 2, Fujifilm GFX × 0,79. Deshalb gibt es z. B. bei Canon und Sony je ein Profil für APS-C und für Vollformat."),
  h2("7.1 Neue Kameras hinzufügen"),
  absatz("Neue Kameras kommen als Profildatei mit der Endung ", fett(".v3cam"), " oder per Update. ", fett("Doppelklick"), " auf die Datei genügt – oder Menü ", fett("Kameras → Kameraprofil importieren …"), "."),
  h2("7.2 Neigungsdaten der Kamera"),
  absatz("Manche Kameras (iPhone, Leica M11/Q3/SL3/D-Lux 8, Lumix L10 u. a.) speichern beim Fotografieren ihre Neigungswinkel in der Bilddatei. Diese Werte nutzt das Zusatzprogramm ", fett("Kameraneigung"), " (Kapitel 8.6). Für Nikon, Canon und OM System wird dazu zusätzlich das kostenlose Programm exiftool benötigt – die Einrichtung beschreibt die Kurzanleitung des Zusatzes (LIESMICH-Kameraneigung.txt)."),
);

// 8 Zusatzprogramme
inhalt.push(h1("8. Zusatzprogramme"),
  h2("8.1 Allgemeines"),
  punkt(fett("Aufruf: "), "Bild in Tri-Vertical öffnen und korrigieren, dann im Menü ", fett("Datei"), " den Zusatz wählen, den Knopf im Bedienfeld anklicken oder das Tastenkürzel drücken. Das Foto wird in voller Auflösung samt Korrektur, Drehung und Spiegelung übernommen."),
  punkt(fett("Weitergeben: "), "Die Zusätze arbeiten zusammen – aus KI-Füllen geht es zu Filter oder Drucken, zwischen Filter und Rahmen mit den Knöpfen „Rahmen …“ bzw. „Filter …“ hin und her, aus beiden zu Drucken."),
  punkt(fett("Übersicht und Entfernen: "), fett("Hilfe → Zusatzprogramme …"), " (Kapitel 2.5)."),

  h2("8.2 Tri-Vertical Drucken"),
  absatz("Druckt das korrigierte Foto in der gewünschten Größe. Die Vorschau links zeigt das Blatt maßstäblich."),
  ...bild(B + "drucken.png", 560, "Druckfenster"),
  tabelle(["Bereich", "Möglichkeiten"], [
    ["Drucker und Papier", "Drucker, Papierformat aus dem Druckertreiber (auch randlose Fotoformate), Ausrichtung; „Druckereinstellungen …“ für Papiersorte und Qualität"],
    ["Bildgröße", "maximal, randlos, feste Fotoformate oder eigene Größe; „Ganzes Foto zeigen“ oder „Format füllen“ (Ausschnitt in der Vorschau verschieben)"],
    ["Ausgabe", "Anzahl der Kopien; unten stehen Blatt- und Fotogröße sowie die Druckauflösung (dpi)"],
  ], [2400, 7238]),

  h2("8.3 Tri-Vertical Rahmen"),
  absatz("Setzt das Foto in einen Rahmen, auf Wunsch mit Beschriftung, und speichert das Ergebnis in voller Auflösung oder gibt es an „Drucken“ weiter."),
  ...bild(B + "rahmen.png", 520, "Rahmenfenster mit Rahmen „Galerie“"),
  tabelle(["Einstellung", "Möglichkeiten"], [
    ["Rahmen-Art", "19 Rahmen: Linie, Doppellinie, Passepartout (mit feiner grauer Kante oder ohne Linie), Galerie, Schatten, abgerundet, Holz hell, Nussbaum, Schwarz, Weiß, Gold, Museum, Polaroid, Dia, Filmstreifen, Büttenrand, unscharfer Hintergrund u. a.; Linienstärke und -farbe"],
    ["Beschriftung", "eigener Text, Dateiname, Aufnahmedatum, Kamera und Objektiv; Schriftgröße"],
    ["Wirkt wie", "Rahmenbreite und Schrift werden so bemessen, als würde das Bild in dieser Größe gedruckt"],
  ], [2400, 7238]),

  h2("8.4 Tri-Vertical Filter"),
  absatz("Gibt dem Foto einen Bildlook und erlaubt die Feinabstimmung. Die Vorschau aktualisiert sich sofort."),
  ...bild(B + "filter.png", 560, "Filterfenster"),
  tabelle(["Gruppe", "Beispiele"], [
    ["Farbe", "Lebendig, Weich & hell, Herbst, Nordisch, Dramatisch, HDR-Look"],
    ["Kamera-Looks", "Natürlich, Analogkino, Zeitgenössisch, Intensiv, Diafilm, Bleichbad …"],
    ["Farbnegativ", "Porträtfilm 160/400, Alltagsfilm 200, Brillantnegativ 100, Kunstlichtfilm 500T, Abgelaufener Film …"],
    ["Diafilm", "Landschaftsdia 50, Neutraldia 100, Kühldia 100, Klassikdia 64, Cross"],
    ["Sofortbild, Retro & Kino", "Sofortbild warm/kühl/S/W, Kino, Vintage, Matt …"],
    ["Schwarzweiß / -film", "klassisch, kontrastreich, Rotfilter, 400 körnig, 100 feinkörnig, gepusht 3200, Infrarot, Orthochromatisch"],
    ["Edeldruck & Tonung", "Sepia, Selen, Palladium, Platin, Van-Dyke-Braun, Albumin, Salzpapier, Cyanotypie"],
    ["Eigene Filter", "eigene Einstellungen mit „Als eigenen Filter speichern …“ sichern"],
  ], [2400, 7238]),
  luecke(),
  absatz("Mit ", fett("Stärke"), " (0–100 %) festlegen, wie stark der Filter wirkt; unter ", fett("Feinabstimmung"), " Belichtung, Kontrast, Lichter, Tiefen, Sättigung, Wärme, Klarheit, Vignette und Körnung nachregeln."),

  h2("8.5 Tri-Vertical KI-Füllen"),
  absatz("Beim Ausrichten wird das Foto verzerrt (z. B. trapezförmig). Statt es zuzuschneiden, behält KI-Füllen das Format des Originals und lässt die leeren Ränder von einer KI passend zum Bild ergänzen."),
  ...bild(B + "ki.png", 560, "Nach dem Öffnen: die leeren Ränder sind kariert dargestellt"),
  ...schritte([
    "Bild in Tri-Vertical öffnen und ausrichten.",
    [fett("Datei → Ränder mit KI füllen …"), " oder Strg+K."],
    "Offline oder Online wählen und auf „Ränder mit KI füllen“ klicken.",
    ["Ergebnis in der Ansicht ", fett("Vergleich"), " prüfen und speichern – oder an Filter bzw. Drucken weitergeben."],
  ]),
  tabelle(["", "Offline (Standard)", "Online (Stability AI)"], [
    ["Kosten", "kostenlos", "etwa 5 Cent pro Bild"],
    ["Datenschutz", "Foto verlässt den PC nicht", "Foto wird hochgeladen"],
    ["Stärken", "Himmel, Fassaden, Wiesen, Pflaster", "auch angeschnittene Gegenstände am Rand"],
    ["Voraussetzung", "einmaliges Laden des KI-Modells (93 MB)", "Konto mit Guthaben und API-Schlüssel"],
  ], [1800, 3919, 3919]),
  luecke(),
  kasten("Bitte beachten", "Die gefüllten Ränder sind von der KI erzeugt und zeigen nicht die Wirklichkeit. Für Dokumentationsfotos lieber zuschneiden."),

  h2("8.6 Tri-Vertical Kameraneigung"),
  absatz("Übernimmt die Neigungswinkel, die die Kamera in der Bilddatei gespeichert hat, beim Öffnen automatisch als Roll und Pitch – das Bild ist sofort korrigiert. Der Knopf ", fett("Kameraneigung"), " stellt die Kamerawerte wieder her. Hat man bereits Regler verstellt, überschreibt der Zusatz nichts."),
  absatz(fett("Stapelverarbeitung: "), "Menü Datei → „Stapelverarbeitung mit Kameraneigung …“. Ordner wählen – jedes Bild wird mit seinen eigenen Winkeln korrigiert und im Unterordner „korrigiert“ gespeichert, in der gewählten Ausgabegröße. Bilder ohne Neigungsdaten werden übersprungen."),
  ...bild(B + "neigung.png", 380, "Stapelverarbeitung mit Kameraneigung"),
  kasten("Originalfotos verwenden", "Messenger und manche Übertragungswege vom Handy entfernen die Kameradaten. Fotos per USB-Kabel oder als „Original“ aus der Cloud übertragen."),
);

// 9 Updates
inhalt.push(h1("9. Updates"),
  absatz("Neue Funktionen und Fehlerbehebungen kommen als kleine Update-Datei mit der Endung ", fett(".v3update"), "."),
  ...schritte([
    ["Doppelklick auf die Update-Datei – oder in Tri-Vertical ", fett("Hilfe → Update installieren …"), "."],
    "Tri-Vertical zeigt die installierte und die neue Version sowie die Neuerungen.",
    ["Mit ", fett("Ja"), " bestätigen. Das Update wird eingespielt und Tri-Vertical startet neu."],
  ]),
  kasten("Sicherheit", "Vor jedem Update wird der bisherige Stand gesichert. Gefällt ein Update nicht: Hilfe → „Letztes Update rückgängig machen“. Nur Update-Dateien aus vertrauenswürdiger Quelle installieren."),
  luecke(),
  absatz(fett("Zusatzprogramme"), " werden mit ihrem neuen Setup aktualisiert – einfach über die alte Version installieren. Die installierte Version steht oben im Bedienfeld und unter ", fett("Hilfe → Über Tri-Vertical"), "."),
);

// 10 Tastenkürzel
inhalt.push(h1("10. Tastenkürzel und Maus"),
  h2("10.1 Tastatur"),
  tabelle(["Taste", "Funktion"], [
    ["Strg + O / Strg + S", "Bild öffnen / Speichern unter"],
    ["A", "Automatisch ausrichten"],
    ["Umschalt + A", "Automatisch frontal ausrichten"],
    ["Strg + ← / Strg + →", "90° nach links / rechts drehen"],
    ["V", "Vorher / Nachher ein/aus"],
    ["Leertaste", "Original zeigen ein/aus"],
    ["K", "Erkannte Kanten (grüne Punkte) ein/aus"],
    ["+  /  −", "Vergrößern / verkleinern"],
    ["0  /  1", "Ganzes Bild / 100 %"],
    ["Strg + P / R / F / K", "Drucken / Rahmen / Filter / KI-Füllen (Zusatzprogramme)"],
  ], [3000, 6638]),
  luecke(),
  h2("10.2 Maus"),
  tabelle(["Maus", "Funktion"], [
    ["Mausrad über dem Bild", "Zoomen an der Mausposition"],
    ["Linke Maustaste ziehen (Bild vergrößert)", "Ausschnitt verschieben"],
    ["Mausrad über einem Regler", "Wert in 0,1°-Schritten, mit Umschalt-Taste in 1°-Schritten"],
    ["Klick auf eine Zahl im Bedienfeld", "Wert genau eintippen"],
    ["Mausrad über dem Bedienfeld", "Bedienfeld scrollen"],
    ["Doppelklick auf .v3cam / .v3update", "Kameraprofil importieren / Update installieren"],
  ], [4200, 5438]),
);

// 11 Problemlösung
const faq = [
  ["„Der Computer wurde durch Windows geschützt“ beim Setup",
    "Auf „Weitere Informationen“ und dann „Trotzdem ausführen“ klicken. Die Meldung erscheint bei nicht kostenpflichtig signierten Programmen."],
  ["Die Automatik findet nichts oder meldet „unsicher“",
    "Sie braucht Dinge, die in Wirklichkeit senkrecht sind (Gebäude, Möbel, Türen, Bretter). Bei Landschaften, Wald oder Porträts ändert sie bewusst nichts – dann die Regler von Hand verwenden. Die grünen Punkte zeigen, woran ausgerichtet wurde."],
  ["Die Senkrechten stimmen, aber das Bild wirkt gestaucht oder der Pitch nicht ganz richtig",
    "Die Brennweite prüfen. Fehlen die Kameradaten, rechnet Tri-Vertical mit 28 mm. Den richtigen Wert am Regler „Brennweite“ einstellen und erneut A drücken."],
  ["Hochhaus: die Automatik korrigiert nur teilweise",
    "Die volle Korrektur würde das Bild extrem verzerren. Den vollen Wert nennt die Statuszeile; bei Bedarf am Regler „Pitch“ weiter korrigieren."],
  ["Mit dem Zusatz Kameraneigung werden keine Neigungsdaten gefunden",
    "Wurde das Foto per Messenger oder Smartphone-Link übertragen? Dann fehlen die Kameradaten – Original per USB-Kabel übertragen. Bei Nikon, Canon und OM System wird exiftool benötigt."],
  ["Ein Zusatzprogramm erscheint nicht im Menü",
    "Tri-Vertical nach der Installation neu starten. Unter Hilfe → Zusatzprogramme prüfen, ob es installiert ist."],
  ["Ein Zusatzprogramm lässt sich in den Windows-Einstellungen nicht deinstallieren",
    "In Tri-Vertical Hilfe → Zusatzprogramme → „Ausgewähltes Zusatzprogramm entfernen …“ verwenden (Kapitel 2.5)."],
  ["Ein gedrehtes Bild erscheint im Zusatzprogramm ungedreht",
    "Das Zusatzprogramm ist zu alt. Die aktuellen Setups installieren (Drucken 2.2, Rahmen 1.5, Filter 1.5, KI-Füllen 1.2, Kameraneigung 1.3)."],
  ["KI-Füllen: Das KI-Modell lässt sich nicht laden",
    "Internetverbindung prüfen und erneut versuchen. Die Fehlermeldung nennt die Adresse des Modells und den Ordner, in den es alternativ von Hand gelegt werden kann."],
  ["„Winkel zu groß“ oder „Korrektur zu stark“",
    "Die eingestellten Winkel sind für diese Brennweite zu extrem. Winkel verkleinern oder Brennweite prüfen."],
];
inhalt.push(h1("11. Problemlösung"));
faq.forEach(([f, a]) => inhalt.push(
  new Paragraph({ spacing: { before: 160, after: 60 }, keepNext: true, children: [text(f, { bold: true, color: FARBE })] }),
  absatz(a)));
// ---------------------------------------------------------------- Dokument
const doc = new Document({
  creator: "Tri-Vertical",
  title: "Tri-Vertical – Bedienungsanleitung",
  description: "Bedienungsanleitung für Tri-Vertical 3.8",
  styles: {
    default: { document: { run: { font: "Arial", size: 21 } } },
    paragraphStyles: [
      { id: "Heading1", name: "Heading 1", basedOn: "Normal", next: "Normal", quickFormat: true,
        run: { size: 34, bold: true, font: "Arial", color: FARBE },
        paragraph: { spacing: { before: 120, after: 240 }, outlineLevel: 0,
          border: { bottom: { style: BorderStyle.SINGLE, size: 12, color: ROT, space: 4 } } } },
      { id: "Heading2", name: "Heading 2", basedOn: "Normal", next: "Normal", quickFormat: true,
        run: { size: 26, bold: true, font: "Arial", color: FARBE },
        paragraph: { spacing: { before: 280, after: 120 }, outlineLevel: 1, keepNext: true } },
    ],
  },
  numbering: {
    config: [
      { reference: "punkte", levels: [{ level: 0, format: LevelFormat.BULLET, text: "•", alignment: AlignmentType.LEFT,
        style: { paragraph: { indent: { left: 540, hanging: 300 } }, run: { color: ROT } } }] },
      ...schrittListen.map((ref) => ({ reference: ref, levels: [{ level: 0, format: LevelFormat.DECIMAL, text: "%1.",
        alignment: AlignmentType.LEFT, style: { paragraph: { indent: { left: 540, hanging: 360 } }, run: { bold: true, color: ROT } } }] })),
    ],
  },
  sections: [{
    properties: { page: { size: { width: 11906, height: 16838 }, margin: { top: 1134, bottom: 1134, left: 1134, right: 1134 } } },
    headers: { default: new Header({ children: [new Paragraph({ alignment: AlignmentType.RIGHT,
      children: [text("TRI-VERTICAL  ·  BEDIENUNGSANLEITUNG", { size: 16, color: "888888" })] })] }) },
    footers: { default: new Footer({ children: [new Paragraph({ alignment: AlignmentType.CENTER,
      children: [text("Seite ", { size: 16, color: "888888" }), new TextRun({ children: [PageNumber.CURRENT], size: 16, color: "888888" }),
        text(" von ", { size: 16, color: "888888" }), new TextRun({ children: [PageNumber.TOTAL_PAGES], size: 16, color: "888888" })] })] }) },
    children: inhalt,
  }],
});

Packer.toBuffer(doc).then((b) => { fs.writeFileSync("Tri-Vertical-Bedienungsanleitung.docx", b); console.log("geschrieben", b.length); });
