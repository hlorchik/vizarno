from pathlib import Path
from docx import Document
from docx.table import Table
from docx.text.paragraph import Paragraph
from docx.oxml.table import CT_Tbl
from docx.oxml.text.paragraph import CT_P


def blocks(document):
    for child in document.element.body.iterchildren():
        if isinstance(child, CT_P):
            yield Paragraph(child, document)
        elif isinstance(child, CT_Tbl):
            yield Table(child, document)


def escape_cell(value):
    return value.replace("|", "\\|").replace("\n", " ").strip()


def convert(source, destination):
    document = Document(source)
    lines = []
    for block in blocks(document):
        if isinstance(block, Paragraph):
            text = block.text.strip()
            if not text:
                continue
            style = block.style.name
            if style == "Title":
                lines.extend(["# " + text, ""])
            elif style.startswith("Heading"):
                level = int(style.rsplit(" ", 1)[-1]) + 1
                lines.extend(["#" * level + " " + text, ""])
            elif style.startswith("List Bullet"):
                lines.append("- " + text)
            elif style.startswith("List Number"):
                lines.append("1. " + text)
            else:
                lines.extend([text, ""])
        else:
            rows = [[escape_cell(cell.text) for cell in row.cells] for row in block.rows]
            if not rows:
                continue
            lines.append("| " + " | ".join(rows[0]) + " |")
            lines.append("| " + " | ".join("---" for _ in rows[0]) + " |")
            for row in rows[1:]:
                lines.append("| " + " | ".join(row) + " |")
            lines.append("")
    destination.write_text("\n".join(lines).strip() + "\n", encoding="utf-8")


if __name__ == "__main__":
    import sys
    convert(Path(sys.argv[1]), Path(sys.argv[2]))
