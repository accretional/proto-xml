# Integration tasks — XML-bearing file formats

Candidates for future `proto-xml` integrations, grouped by genre. Each
entry notes where XML sits inside the format, a pointer to the spec,
and anything unusual about how the XML is used (relevant when building
a parser or codec on top).

The big family we'd plug into first is **Open Packaging Convention
(OPC / OCF)** — a ZIP of XML parts plus a manifest. DOCX, XLSX, PPTX,
and EPUB all belong to it, so a single "container + XML payloads"
integration unlocks the whole genre.

---

## Open Packaging family (ZIP of XML)

### EPUB
- **Spec.** <https://www.w3.org/TR/epub-33/>
- **XML usage.** `META-INF/container.xml` points at an OPF "package
  document" (XML). The package document references XHTML spine items,
  an NCX / Navigation Document (XML/XHTML), and metadata in Dublin
  Core. Already the user's home turf.

### DOCX — WordprocessingML
- **Spec.** ECMA-376. <https://www.ecma-international.org/publications-and-standards/standards/ecma-376/>
- **XML usage.** `word/document.xml` is the main content; `styles.xml`,
  `numbering.xml`, `settings.xml`, `footnotes.xml`, and
  `word/_rels/*.rels` tie everything together. The relationships XML
  (`.rels`) is half of OPC. Already the user's home turf.

### XLSX — SpreadsheetML
- **Spec.** ECMA-376. <https://www.ecma-international.org/publications-and-standards/standards/ecma-376/>
- **XML usage.** `xl/workbook.xml` defines sheets; each
  `xl/worksheets/sheetN.xml` is a dense cell grid; `sharedStrings.xml`
  deduplicates text; `styles.xml` carries formatting. Formulas are
  strings with cell references. Parsing gets awkward because cell text
  is indirected through shared strings.

### PPTX — PresentationML
- **Spec.** ECMA-376.
- **XML usage.** `ppt/presentation.xml` plus one XML per slide under
  `ppt/slides/`. Shape trees with DrawingML (shared with DOCX/XLSX).
  Animations and transitions are inline XML.

### ODT / ODS / ODP — OpenDocument Format
- **Spec.** OASIS ODF. <https://www.oasis-open.org/standards/#opendocumentv1.3>
- **XML usage.** ZIP with `content.xml`, `styles.xml`,
  `meta.xml`, `settings.xml`, `manifest.xml`. Cleaner, more orthogonal
  schema than OOXML; same document/spreadsheet/presentation split. A
  good second integration after DOCX because the codec shape matches.

### VSDX — Visio Drawing
- **Spec.** Microsoft Open Specifications MS-VSDX.
- **XML usage.** OPC ZIP. Shapes, pages, masters each in their own XML
  part; connector geometry is XML.

### FB2 — FictionBook
- **Spec.** <http://www.fictionbook.org/>
- **XML usage.** Single XML file (not zipped), embeds binary image
  data as base64 inside the XML. Popular for Russian-language ebooks.

### DAISY — digital talking books
- **Spec.** <https://daisy.org/activities/standards/daisy/>
- **XML usage.** ZIP with SMIL timing files, DTBook XML content, and
  audio clips. Accessibility-oriented counterpart to EPUB.

---

## Vector / drawing / 3D

### SVG — Scalable Vector Graphics
- **Spec.** <https://www.w3.org/TR/SVG2/> (also SVG 1.1 widely deployed).
- **XML usage.** The format *is* XML — element tree maps directly to
  shape primitives, paths, text runs, gradients, and filters. Inline
  `<script>` and `<style>` are allowed. Big real-world corpus on every
  website.

### MathML
- **Spec.** <https://www.w3.org/TR/MathML3/>
- **XML usage.** Pure XML. Presentation MathML (rendered) and Content
  MathML (semantic) are two parallel vocabularies. Appears inside
  EPUB, DOCX (via `omml` → MathML), JATS.

### COLLADA — 3D asset interchange
- **Spec.** Khronos. <https://www.khronos.org/collada/>
- **XML usage.** `.dae` file is single XML: geometry, materials,
  scenes, animations, physics. Used for asset pipelines (Blender,
  Unity, Unreal exporters).

### X3D — web 3D (VRML successor)
- **Spec.** Web3D Consortium. <https://www.web3d.org/x3d/what-x3d>
- **XML usage.** Scene graph in XML. Runs alongside X3D-JSON and the
  classic VRML-style encoding.

### PLMXML / IFC-XML
- **Spec.** IFC-XML (buildingSMART) <https://technical.buildingsmart.org/standards/ifc/ifc-formats/>
- **XML usage.** IFC-XML is the XML serialisation of the IFC building
  model (BIM). Huge files with deep refs between objects. Geometry
  pipelines typically prefer the STEP-style encoding, but regulators
  increasingly demand XML.

---

## Publishing / structured authoring

### DocBook
- **Spec.** <https://docbook.org/>
- **XML usage.** Long-form technical authoring. One of the oldest XML
  vocabularies; huge toolchain (xsltproc, DocBook XSL) for rendering
  to HTML/PDF.

### DITA — Darwin Information Typing Architecture
- **Spec.** OASIS DITA. <https://www.oasis-open.org/committees/tc_home.php?wg_abbrev=dita>
- **XML usage.** Modular topics + maps. Each topic is a small XML
  file; maps reference topics to build books. Common in enterprise
  technical-writing shops.

### TEI — Text Encoding Initiative
- **Spec.** <https://tei-c.org/>
- **XML usage.** Scholarly edition of texts. Heavy markup of variants,
  annotations, structural features. Tons of public digital-humanities
  corpora.

### JATS — Journal Article Tag Suite
- **Spec.** NLM / NISO. <https://jats.nlm.nih.gov/>
- **XML usage.** Every article on PubMed Central ships as JATS XML.
  Nested sections, figures, references, MathML for equations. Largest
  freely-available scientific XML corpus.

### XLIFF — localization interchange
- **Spec.** OASIS. <https://www.oasis-open.org/committees/xliff/>
- **XML usage.** Translation units (source/target pairs) plus inline
  placeholders preserving formatting from the source format. Every
  translation management system speaks it.

### TMX — Translation Memory eXchange
- **Spec.** GALA. <https://www.gala-global.org/lisa-oscar-standards>
- **XML usage.** Bilingual or multilingual translation-memory
  databases. Dumped straight from CAT tools.

### XMP — Extensible Metadata Platform
- **Spec.** ISO 16684 / Adobe. <https://developer.adobe.com/xmp/docs/>
- **XML usage.** RDF/XML metadata blob embedded inside PDF, JPEG, TIFF,
  MP3, etc. The XML is the metadata; the outer file is anything. A
  format-aware extractor (rather than a pure XML parser) is normally
  needed because you have to find it inside the binary host.

---

## Feeds

### RSS 2.0
- **Spec.** <https://www.rssboard.org/rss-specification>
- **XML usage.** Root `<rss>` with a `<channel>` and `<item>`s. Many
  namespaced extensions (`media:`, `content:encoded`, `dc:`).

### Atom
- **Spec.** RFC 4287. <https://datatracker.ietf.org/doc/html/rfc4287>
- **XML usage.** Successor to RSS. Stricter schema, namespaced.
  Fixture `data/handwritten/10_atom_feed.xml` already in-tree.

---

## Geospatial

### GPX — GPS Exchange Format
- **Spec.** <https://www.topografix.com/gpx.asp>
- **XML usage.** Tracks, waypoints, routes. Tiny schema, huge
  real-world use (every GPS device / fitness app).

### KML — Keyhole Markup Language
- **Spec.** OGC. <https://www.ogc.org/standards/kml>
- **XML usage.** Placemarks, paths, polygons, overlays. Google Earth
  native format. KMZ is a ZIP wrapper around KML.

### GML — Geography Markup Language
- **Spec.** OGC. <https://www.ogc.org/standards/gml>
- **XML usage.** The grandparent of modern geospatial XML. Feature
  collections with coordinates, CRS, topology. Underlies KML, CityGML,
  and ISO 19136.

### CityGML
- **Spec.** OGC. <https://www.ogc.org/standards/citygml>
- **XML usage.** 3D city models in XML. Buildings with LOD1–LOD4,
  terrain, vegetation. Uses GML underneath.

---

## Multimedia / streaming

### TTML — Timed Text Markup Language (subtitles)
- **Spec.** <https://www.w3.org/TR/ttml2/>
- **XML usage.** Subtitle / caption authoring. IMSC profile is used
  by broadcasters; DFXP is a subset used by Netflix-era streaming.

### SMIL — Synchronized Multimedia Integration Language
- **Spec.** <https://www.w3.org/TR/SMIL3/>
- **XML usage.** Timeline of media elements. DAISY uses it for
  text-audio sync; EPUB 3 uses a subset for media overlays.

### MPEG-DASH MPD — streaming manifest
- **Spec.** ISO/IEC 23009-1. <https://www.iso.org/standard/79329.html>
- **XML usage.** Single XML describing adaptation sets, segments, and
  timing for adaptive bitrate streaming. Parsed by every HTML5 video
  player using DASH.

### SCORM — e-learning packages
- **Spec.** ADL. <https://adlnet.gov/projects/scorm/>
- **XML usage.** ZIP with `imsmanifest.xml` (IMS Content Packaging)
  plus HTML/JS course content. LMS vendors all consume it.

---

## Enterprise / messaging / security

### SOAP / WSDL
- **Specs.** <https://www.w3.org/TR/soap12-part1/>,
  <https://www.w3.org/TR/wsdl20/>
- **XML usage.** Envelope with header + body; WSDL describes service
  endpoints and message types. Legacy but still ubiquitous in
  enterprise / government systems.

### SAML — Security Assertion Markup Language
- **Spec.** OASIS. <https://www.oasis-open.org/committees/security/>
- **XML usage.** SSO assertions exchanged between identity and
  service providers. Heavy use of XML Signature and XML Encryption.

### XML Signature / XML Encryption
- **Specs.** <https://www.w3.org/TR/xmldsig-core1/>,
  <https://www.w3.org/TR/xmlenc-core1/>
- **XML usage.** Canonicalised XML fragments signed or encrypted
  in-place. Needed for SAML, many government e-filing systems. This
  is where a real C14N implementation (todo #6) would pay off.

### XBRL — eXtensible Business Reporting Language
- **Spec.** XBRL International. <https://www.xbrl.org/>
- **XML usage.** Financial filings. Public corpus: the entire SEC
  EDGAR system ships in XBRL / inline-XBRL (iXBRL embeds XBRL in
  XHTML). Schemas have ~40k taxonomy elements; parser stress test.

### ISO 20022 / FIX-XML
- **Specs.** <https://www.iso20022.org/>, <https://www.fixtrading.org/>
- **XML usage.** ISO 20022 messages are used by SWIFT for payments;
  FIX is used for trading. Both define huge schemas.

### HL7 CDA / FHIR-XML
- **Specs.** <https://www.hl7.org/implement/standards/product_brief.cfm?product_id=7>,
  <https://www.hl7.org/fhir/>
- **XML usage.** Healthcare interchange. FHIR can be XML or JSON; CDA
  is XML-only and embeds narrative blocks as XHTML.

---

## Config / build / platform (utility, not glamorous)

- **Maven POM.** <https://maven.apache.org/pom.html> — Java build
  descriptor.
- **Ant build.xml**, **.csproj / .vcxproj** — build graphs in XML.
- **web.xml** — Java servlet deployment descriptor.
- **plist (XML form)** —
  <https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/PropertyLists/>
  Apple config. There is also a binary plist variant.
- **AndroidManifest.xml** — Android app manifest (ships *binary* XML
  inside APKs; source form is text XML).

Useful if the user wants coverage for dev-tool use cases, but each is
small and bespoke.

---

## Suggested sequencing

If the goal is to extend `proto-xml` to real document work, the
incremental value ladder would be roughly:

1. **OPC / OCF container support.** A thin layer on top of `proto-xml`
   that unzips a container, exposes each XML part, and keeps the
   relationships graph. Unlocks DOCX, XLSX, PPTX, EPUB, PPTX, ODT.
2. **ODT / ODS / ODP.** Same shape as DOCX/XLSX/PPTX but a cleaner
   schema; good test of whether the container layer is generic.
3. **SVG.** Huge corpus, self-contained, tests non-document XML.
4. **JATS.** Deep nesting + MathML, largest public scientific corpus.
5. **KML / GPX.** Easy wins, high user visibility (maps, fitness).
6. **XBRL / SEC EDGAR.** Stress test for very large schemas and
   heavy refs.

Anything below that is situational — pull in if a concrete user asks.
