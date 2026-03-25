# Static Project Site

This directory contains a simple static landing page for `chatgpt-bulk`.

## Preview locally

No build step is required.

Quick check by opening the file directly:

```bash
open site/index.html
```

Or serve it locally so links and assets behave like a hosted site:

```bash
python3 -m http.server 8000 --directory site
```

Then visit `http://localhost:8000`.

## Hosting

The site is plain HTML and CSS, so it can be published from this directory with
GitHub Pages or any other static host.
