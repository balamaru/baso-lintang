---
name: Kamus Lintang
description: Kamus komunitas berbahasa Indonesia, dirancang untuk pencarian dan kontribusi lewat ponsel.
colors:
  primary: "#f2c14e"
  night: "#0f1b3d"
  night-deep: "#0a1330"
  surface: "#121f47"
  text: "#e8edfa"
  secondary-text: "#a3afd0"
  line: "#26366b"
  verified: "#62c9b6"
  white: "#ffffff"
  search-ink: "#14203f"
  sky-copy: "#c9d3f0"
  star-hint: "#9fb0e0"
  footer-copy: "#b8c4e8"
  menu-line: "#5b6aa0"
  deep-copy: "#56627f"
  hover-blue: "#233363"
  focus-blue: "#5f75a8"
  success-surface: "#133541"
  error-surface: "#40233b"
  error-line: "#f3a6a6"
  error-copy: "#ffe0e0"
  action-hover: "#ffcf63"
typography:
  display:
    fontFamily: "Bricolage Grotesque, Trebuchet MS, system-ui, sans-serif"
    fontSize: "clamp(2.3rem, 6vw, 4rem)"
    fontWeight: 700
    lineHeight: 1.12
  body:
    fontFamily: "Source Sans 3, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 400
    lineHeight: 1.6
  section:
    fontFamily: "Bricolage Grotesque, Trebuchet MS, system-ui, sans-serif"
    fontSize: "clamp(1.7rem, 4vw, 2.4rem)"
    fontWeight: 700
    lineHeight: 1.12
  page-title:
    fontFamily: "Bricolage Grotesque, Trebuchet MS, system-ui, sans-serif"
    fontSize: "clamp(2rem, 4vw, 2.75rem)"
    fontWeight: 700
    lineHeight: 1.12
  body-large:
    fontFamily: "Source Sans 3, system-ui, sans-serif"
    fontSize: "1.15rem"
    fontWeight: 400
    lineHeight: 1.6
  control-copy:
    fontFamily: "Source Sans 3, system-ui, sans-serif"
    fontSize: "1.1rem"
    fontWeight: 600
    lineHeight: 1.5
  small:
    fontFamily: "Source Sans 3, system-ui, sans-serif"
    fontSize: ".95rem"
    fontWeight: 400
    lineHeight: 1.5
  subsection:
    fontFamily: "Bricolage Grotesque, Trebuchet MS, system-ui, sans-serif"
    fontSize: "1.2rem"
    fontWeight: 700
    lineHeight: 1.12
  entry-title:
    fontFamily: "Bricolage Grotesque, Trebuchet MS, system-ui, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 700
    lineHeight: 1.12
  tag:
    fontFamily: "Source Sans 3, system-ui, sans-serif"
    fontSize: ".85rem"
    fontWeight: 400
    lineHeight: 1.5
  compact-copy:
    fontFamily: "Source Sans 3, system-ui, sans-serif"
    fontSize: ".9rem"
    fontWeight: 400
    lineHeight: 1.5
  dialog-title:
    fontFamily: "Bricolage Grotesque, Trebuchet MS, system-ui, sans-serif"
    fontSize: "1.6rem"
    fontWeight: 700
    lineHeight: 1.12
  empty-title:
    fontFamily: "Bricolage Grotesque, Trebuchet MS, system-ui, sans-serif"
    fontSize: "1.4rem"
    fontWeight: 700
    lineHeight: 1.12
rounded:
  control: "10px"
  card: "16px"
  compact: "8px"
  notice: "12px"
  search: "14px"
  chip: "99px"
spacing:
  page-gutter: "20px"
  section-desktop: "68px"
  section-mobile: "48px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.night}"
    rounded: "{rounded.control}"
    padding: "11px 20px"
  field:
    backgroundColor: "{colors.night-deep}"
    textColor: "{colors.text}"
    rounded: "{rounded.control}"
    padding: "12px 14px"
---

## Overview

**Creative North Star: The Penutur's Open Dictionary.** A calm, night-sky reading space lets people search first and contribute without an account. The dark blue field gives long-form Indonesian text a steady home; warm yellow marks the main action, while teal is reserved for verified status. This visual system supports community review without presenting unreviewed submissions as established knowledge.

**Key Characteristics:**
- Search and contribution remain prominent at phone widths.
- A locally hosted display and body face give headings character while keeping paragraphs easy to read.
- Verification is identified with restrained teal labels.
- Decorative stars are limited to the hero; the remaining surfaces are flat and quiet.

**The Public-State Rule.** A verified tag belongs only to published entries; pending submissions stay in the review workflow.

## Colors

Deep blue surfaces create a consistent reading environment. Yellow is the action accent, and teal communicates verification.

### Primary
- **Warm Star** (#f2c14e): Main search and submit actions, visible keyboard focus, and small brand accents.

### Neutral
- **Night Ink** (#0f1b3d): Header, footer, and text on yellow actions.
- **Deep Night** (#0a1330): Page and field background.
- **Dictionary Panel** (#121f47): Form panels and team workspace surfaces.
- **Reading White** (#e8edfa): Primary text on dark surfaces.
- **Quiet Blue** (#a3afd0): Supporting text and secondary labels.
- **Blue Rule** (#26366b): Dividers and field outlines.
- **Verified Teal** (#62c9b6): Confirmed-entry status.

**The Status Color Rule.** Teal denotes verification; yellow denotes an action or focus.

## Typography

**Display Font:** Bricolage Grotesque (with Trebuchet MS and system sans-serif fallbacks)
**Body Font:** Source Sans 3 (with system sans-serif fallbacks)

**Character:** The display face gives short headings a distinct voice; Source Sans 3 keeps longer instructions clear. Both fonts are served from the site itself.

### Hierarchy
- **Display** (700, clamp(2.3rem, 6vw, 4rem), 1.12): Homepage hero headline.
- **Headline** (700, clamp(1.7rem, 4vw, 2.4rem), 1.12): Section headings.
- **Title** (700, 1.5rem, 1.12): Entry names and compact page headings.
- **Body** (400, 1.0625rem, 1.6): Main copy and form content.
- **Label** (600, 1rem): Form labels and controls.

**The Two-Voice Rule.** Use the display face for headings and the body face for instructions, labels, and definitions.

## Layout

The main content stays within a 1040px reading frame with 20px side gutters. Sections have generous desktop separation and contract on small screens. Forms use two columns where space allows and one column below 700px. Navigation collapses to a keyboard-operable mobile menu below 760px. Entry results use a compact two-column list on wider screens and stack on phones.

## Elevation & Depth

Surfaces are flat at rest. Separation comes from a darker or lighter blue plane and a thin blue rule; there is no general card shadow vocabulary. The hero uses a restrained radial night-sky field and small star marks as its one atmospheric surface.

**The Flat Surface Rule.** Use tonal contrast and borders to separate content; do not add decorative shadows.

## Shapes

Controls use gently rounded corners (10px); larger panels use a broader radius (16px). Entry lists use horizontal rules rather than nested cards. Focus is shown with a clear 3px yellow outline and offset.

## Components

### Buttons
- **Shape:** Softly rounded (10px).
- **Primary:** Warm yellow with night-ink text, 11px by 20px padding, and semibold type.
- **Hover / Focus:** Darken or strengthen the yellow; keyboard focus keeps a 3px visible outline.
- **Secondary:** Transparent with a visible outline.

### Cards / Containers
- **Corner Style:** Broad, 16px radius for form panels.
- **Background:** Dictionary Panel against Deep Night.
- **Shadow Strategy:** Flat; separation uses the surface tone and a thin Blue Rule.
- **Border:** One-pixel Blue Rule.
- **Internal Padding:** 28px on desktop and 20px on mobile.

### Inputs / Fields
- **Style:** Deep Night fill, one-and-a-half-pixel Blue Rule, 10px corners, and 12px by 14px padding.
- **Focus:** Visible 3px Warm Star outline with offset.
- **Error:** A distinct error notice accompanies preserved form values.

### Navigation
- **Style:** Sticky Night Ink bar with a locally rendered wordmark and simple links.
- **Mobile:** Toggleable links remain in the document and can be operated by keyboard.

## Do's and Don'ts

### Do:
- **Do** keep the search and send-word actions easy to find at phone widths.
- **Do** use teal only for verified status and yellow for actions or focus.
- **Do** preserve the flat blue surfaces and readable local font pairing.

### Don't:
- **Don't** style pending submissions as verified dictionary entries.
- **Don't** add decorative shadows or external font and analytics services.
- **Don't** fill the dictionary with illustrative language presented as real Lintang vocabulary.
