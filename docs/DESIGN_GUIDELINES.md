# avyos Design Guidelines

## Philosophy

avyos follows the **Dharmic Design** system — an intersection of Indian Hindu traditional aesthetics and modern software design principles. Every visual decision is rooted in a cultural archetype: the warmth of a temple lamp, the precision of a kolam pattern, the richness of temple silk.

The result is an interface that feels simultaneously ancient and contemporary: deeply rooted, visually rich, and functionally sharp.

---

## Color System

### Sacred Palette

All colors trace their lineage to traditional Indian dyes, pigments, and sacred materials.

| Token | Light Value | Dark Value | Origin |
|---|---|---|---|
| `--accent` | `#C94B0A` | `#FF7D22` | Kesari — saffron flag dye |
| `--accent-2` | `#C9941E` | `#F5C842` | Swarna — temple gold |
| `--bg` | `#FDF4E3` | `#0C0812` | Parchment / Night indigo |
| `--bg-alt` | `#FEF9EE` | `#160E20` | Cream / Deep maroon |
| `--text` | `#1C0D00` | `#F5E8CC` | India ink / Warm cream |
| `--muted` | `#7A5230` | `#C4996A` | Sandalwood / Aged brass |
| `--border` | `rgba(201,148,48,.28)` | `rgba(201,148,48,.22)` | Gold leaf |

### Semantic Meaning

- **Saffron (Kesari)** — primary action, active states, links. Represents courage and sacrifice.
- **Gold (Swarna)** — secondary accent, highlights, decorative borders. Represents prosperity.
- **Sandalwood (Chandan)** — muted text, subdued labels. Represents calm and groundedness.
- **Night Indigo** — dark-mode base. Evokes the pre-dawn temple sky before morning prayers.
- **Warm Cream** — dark-mode text. Evokes candlelight and lamp glow.

### Do not use

- Pure cold blue (`#0000ff`) — clashes with the warm palette
- Harsh neon greens — inconsistent with the organic, earthy tone
- Pure black or white backgrounds — use parchment and night indigo instead

---

## Typography

### Typefaces

| Role | Font | Why |
|---|---|---|
| Display / Headings | **Noto Serif** | Elegant serifs that reference the proportions of Devanagari script; multi-script heritage |
| Body | **Noto Sans** | Clean, readable, same family as Noto Serif for harmonic pairing |
| Code / Mono | **JetBrains Mono** | Technical clarity; generous x-height for code readability |

### Type Scale

| Name | Size | Usage |
|---|---|---|
| Hero | `clamp(2.1rem, 6vw, 3.4rem)` | Page title on landing |
| Section | `clamp(1.85rem, 4vw, 2.45rem)` | Major section headers |
| H1 | `clamp(1.32rem, 1.9vw, 1.7rem)` | Doc page title |
| H2 | `clamp(1.14rem, 1.6vw, 1.42rem)` | Section within a doc |
| Body | `18px` | Base reading size |
| Small | `0.86rem` | Captions, labels, code in prose |
| Micro | `0.72–0.76rem` | Nav labels, tags, monospace badges |

### Rules

- Headings use `font-family: var(--display)` (Noto Serif) with `letter-spacing: -0.02em`
- Body uses `font-family: var(--body)` (Noto Sans) with `line-height: 1.55`
- Code uses `font-family: var(--mono)` (JetBrains Mono) with `font-size: 0.86rem`
- Never mix Noto Serif and another serif; never mix two display fonts

---

## Decorative Elements

### Tilak Bar

A 3px horizontal stripe at the very top of every page — a digital tilak. Uses a saffron-to-gold-to-saffron gradient, evoking the sacred mark worn at the forehead.

```css
/* Implemented via body::before */
background: linear-gradient(90deg, #B5451B, #E8690A, #F5C842, #E8690A, #B5451B);
```

### Dot Pattern (Rangoli / Kolam)

A 28×28px repeating dot grid at very low opacity underlays the page body. Inspired by the dot framework used to draw kolam and rangoli patterns across South India.

```css
background-image: radial-gradient(circle, var(--pattern-dot) 1px, transparent 1px);
background-size: 28px 28px;
```

### OM Symbol (ॐ)

The brand mark includes the Devanagari Om character (ॐ) rendered in a saffron-to-gold gradient. It anchors the brand identity to the project's Hindu cultural inspiration.

### Ornamental Dividers

Section heading borders use a double-rule effect: a solid line with a thin gold shadow beneath, evoking the double-line borders of traditional manuscript pages.

### Gold Border on Cards

All cards use a gold-tinted semi-transparent border (`rgba(201,148,48,...)`) rather than a neutral grey. In dark mode, this creates a subtle gilded effect reminiscent of illuminated manuscripts.

---

## Spacing & Layout

The spacing scale uses the existing `--space-*` tokens unchanged. Layout rules:

- Sidebar: `260–320px` fixed, sticky
- Content: fills remaining space, max comfortable line length ~80ch
- TOC: `220px` on viewports ≥ 1380px
- All grid gaps: `0.68–0.84rem` for dense docs, `1rem–1.5rem` for landing sections

---

## Components

### Buttons

| Variant | Use | Colors |
|---|---|---|
| Primary | Main CTAs | Saffron-to-gold gradient, white text |
| Release / Secondary | Download, Source | Deeper saffron-to-orange, white text |
| Ghost | Tertiary actions | Glass background, border |

Primary button shadow uses `rgba(201,80,10,0.32)` — a warm saffron glow, not cold blue.

### Cards

All cards have:
- `border-radius: var(--radius-xl)` (26px) on landing; `10px` on docs
- Gold-tinted border
- Warm card background (parchment in light, deep maroon in dark)
- Soft warm shadow (`rgba(92,48,12,...)` in light, near-black in dark)

### Code Blocks

- Container: warm dark background in light mode, deep indigo in dark
- Gold-tinted top border (3px) to mark them as "sacred text"
- Copy button appears on hover in the top-right corner
- Language tag inferred by highlight.js

### Navigation Sidebar

- Active links: saffron left-border indicator (`var(--accent)`), warm tinted background
- Hover: translucent warm gold background
- Section labels: all-caps, spaced, sandalwood muted color

### Heading Anchors

Every `h2`/`h3` inside doc content has a `#` anchor link visible on hover. Color: saffron (`var(--accent)`).

---

## Dark Mode

Dark mode is not simply an inverted palette — it is a distinct environment:

- **Background**: deep indigo-maroon (`#0C0812`) — the pre-dawn temple sky
- **Text**: warm cream (`#F5E8CC`) — lamplight on manuscript
- **Accent**: brighter saffron (`#FF7D22`) — a flame in the dark
- **Borders**: gold at lower opacity — gilded in shadow
- **Code blocks**: deep indigo with gold border

Toggle is stored with the site-wide key `"avyos-theme"` using a shared cookie first, with `localStorage` as a compatibility fallback. The old `"docgen-theme"` key is migrated automatically. Default follows `prefers-color-scheme`.

---

## Accessibility

- All text meets WCAG AA contrast on their respective backgrounds
- Focus rings: warm saffron ring (`rgba(201,80,10,0.28)`) — no blue
- Interactive elements have `aria-label` attributes
- Color is never the only indicator of state (shapes and text also change)
- Reduced-motion: `prefers-reduced-motion` respected by all transitions

---

## Voice & Tone (Documentation)

Technical documentation should be:

- **Direct** — state facts, avoid hedging
- **Respectful** — the reader is knowledgeable
- **Precise** — prefer exact terms over vague descriptions
- **Concise** — one idea per paragraph

Avoid:
- Exclamation marks in technical prose
- Marketing language in API docs
- Jargon without definition on first use
