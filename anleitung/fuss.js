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
