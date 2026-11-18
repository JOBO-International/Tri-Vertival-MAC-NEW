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

