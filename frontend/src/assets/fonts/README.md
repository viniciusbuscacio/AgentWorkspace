# Bundled fonts

Each `<id>-<weight>.woff2` here is a latin-subset web font shipped **inside** the
app so the font picker renders identically on macOS, Windows and Linux — it does
**not** depend on the font being installed on the user's machine.

- Files are downloaded by `scripts/fetch-fonts.sh` (Google Fonts, weights 400 & 700).
- They are declared as `@font-face` in `../../theme/fonts.css`.
- They are exposed as picker options in `../../lib/app-font.ts` (`APP_FONT_OPTIONS`)
  and allow-listed for the agent in `internal/infrastructure/tools/aw_app.go`.

See **`docs/FONTS.md`** for the full "how to add a font" procedure.

## Licenses

All bundled families are free to redistribute:

- **OFL-1.1** (SIL Open Font License): Inter, Roboto, Open Sans, Lato, Montserrat,
  Poppins, Nunito, Work Sans, DM Sans, Manrope, Rubik, Merriweather, Lora,
  Playfair Display, JetBrains Mono, Fira Code, IBM Plex Mono, Comic Neue,
  Libre Franklin, Jost, Libre Baskerville, Inconsolata.
- **Apache-2.0**: Arimo, Tinos, Cousine, Gelasio (metric-compatible with
  Arial, Times New Roman, Courier New and Georgia respectively).

`OFL.txt` holds the SIL Open Font License text. Font copyrights remain with
their respective authors.
